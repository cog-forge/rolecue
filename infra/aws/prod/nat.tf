# NAT instance instead of a NAT gateway (~$45/month). One instance, no Elastic IP: egress for the private-app subnets
# only. A single point of failure for outbound traffic, which is acceptable for this workload.

# Latest Amazon Linux 2023 arm64 AMI, resolved from the public SSM parameter. insecure_value keeps the AMI id readable in
# the plan; it is a public identifier, not a secret.
data "aws_ssm_parameter" "al2023_arm64" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
}

# --- Session Manager access ----------------------------------------------------------------------------------------
# The instance has no SSH key and no inbound rule, so Session Manager (and Run Command) is the only way in. It reaches
# the SSM endpoints through the instance's own public IP.

data "aws_iam_policy_document" "ec2_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "nat" {
  name               = "${local.name}-nat"
  description        = "NAT instance: Systems Manager core (Session Manager, Run Command), nothing else"
  assume_role_policy = data.aws_iam_policy_document.ec2_trust.json
}

resource "aws_iam_role_policy_attachment" "nat_ssm" {
  role       = aws_iam_role.nat.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "nat" {
  name = "${local.name}-nat"
  role = aws_iam_role.nat.name
}

# --- The instance --------------------------------------------------------------------------------------------------

resource "aws_instance" "nat" {
  ami                    = nonsensitive(data.aws_ssm_parameter.al2023_arm64.insecure_value)
  instance_type          = "t4g.micro"
  subnet_id              = aws_subnet.public[0].id
  vpc_security_group_ids = [aws_security_group.nat.id]
  iam_instance_profile   = aws_iam_instance_profile.nat.name
  source_dest_check      = false

  metadata_options {
    http_tokens = "required"
  }

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 8
    encrypted             = true
    delete_on_termination = true
  }

  # What went wrong the first time: on a t4g.nano (512 MiB) `dnf install` OOM-killed itself while loading the 90 MB
  # AL2023 repository metadata (console log: "Out of memory: Killed process ... (dnf)", then "part-001: line 4: Killed").
  # `set -e` ended the script there, so IP forwarding and the NAT rule were never configured. The instance is now a
  # t4g.micro (1 GiB), and the disk swapfile before the install stays as a safety margin (the 422 MB zram swap the AMI
  # ships was already full when dnf died).
  #
  # The NAT rule lives in a small systemd unit that runs at every boot, not only at first boot: user data runs once per
  # instance, and the sleep/wake schedule stops and starts this instance. The rule needs no interface name (it matches
  # on addresses), so there is nothing to detect and no race with the default route.
  #
  # Shell variables are written $VAR, never ${VAR}, because Terraform would read ${...} as an interpolation.
  user_data_replace_on_change = true
  user_data                   = <<-EOT
    #!/bin/bash
    set -euxo pipefail

    # Forward packets between the private subnets and the internet, persisted across reboots.
    echo 'net.ipv4.ip_forward = 1' > /etc/sysctl.d/90-nat.conf
    sysctl -p /etc/sysctl.d/90-nat.conf

    # 1 GiB swapfile as headroom for dnf. Kept (fstab) because update-motd runs dnf at every boot too.
    if [ ! -f /swapfile ]; then
      dd if=/dev/zero of=/swapfile bs=1M count=1024
      chmod 600 /swapfile
      mkswap /swapfile
      swapon /swapfile
      echo '/swapfile none swap sw,pri=10 0 0' >> /etc/fstab
    fi

    # nftables is not part of the standard AL2023 image. Retry once or twice in case the repository is briefly slow;
    # the final check fails the script (and shows in the console log) if nft is still missing.
    for attempt in 1 2 3; do
      command -v nft >/dev/null && break
      dnf install -y nftables || sleep 10
    done
    NFT="$(command -v nft)"

    # Masquerade traffic from the VPC to anywhere outside the VPC. Declared, deleted and redeclared so loading the
    # file is idempotent; the counter shows in `nft list ruleset` whether packets are being translated.
    cat > /etc/nat-masquerade.nft <<'NFTEOF'
    table ip rolecue_nat
    delete table ip rolecue_nat
    table ip rolecue_nat {
      chain postrouting {
        type nat hook postrouting priority srcnat; policy accept;
        ip saddr ${local.vpc_cidr} ip daddr != ${local.vpc_cidr} counter masquerade
      }
    }
    NFTEOF

    cat > /etc/systemd/system/nat-masquerade.service <<UNITEOF
    [Unit]
    Description=NAT masquerade for the VPC
    Wants=network-online.target
    After=network-online.target

    [Service]
    Type=oneshot
    RemainAfterExit=yes
    ExecStartPre=-/usr/sbin/sysctl -w net.ipv4.ip_forward=1
    ExecStart=$NFT -f /etc/nat-masquerade.nft
    ExecStop=$NFT delete table ip rolecue_nat

    [Install]
    WantedBy=multi-user.target
    UNITEOF

    systemctl daemon-reload
    systemctl enable --now nat-masquerade.service

    # Proof for `aws ec2 get-console-output`, whether or not SSM has registered yet.
    sysctl net.ipv4.ip_forward
    "$NFT" list ruleset
    echo "NAT-SETUP-OK $(date -u +%FT%TZ)" | tee /dev/console
  EOT

  tags = { Name = "${local.name}-nat" }

  lifecycle {
    # A newer "latest" AL2023 image must not replace a working NAT instance.
    ignore_changes = [ami]
  }
}

# Sleep/wake: the instance is stopped while awake = false, which stops its hourly charge (the root volume is still billed).
resource "aws_ec2_instance_state" "nat" {
  instance_id = aws_instance.nat.id
  state       = var.awake ? "running" : "stopped"
}
