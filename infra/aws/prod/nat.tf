# NAT instance instead of a NAT gateway (~$45/month). One instance, no Elastic IP: egress for the private-app subnets
# only. A single point of failure for outbound traffic, which is acceptable for this workload.

# Latest Amazon Linux 2023 arm64 AMI, resolved from the public SSM parameter. insecure_value keeps the AMI id readable in
# the plan; it is a public identifier, not a secret.
data "aws_ssm_parameter" "al2023_arm64" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
}

resource "aws_instance" "nat" {
  ami                    = nonsensitive(data.aws_ssm_parameter.al2023_arm64.insecure_value)
  instance_type          = "t4g.nano"
  subnet_id              = aws_subnet.public[0].id
  vpc_security_group_ids = [aws_security_group.nat.id]
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

  user_data_replace_on_change = true
  user_data                   = <<-EOT
    #!/bin/bash
    set -euxo pipefail

    dnf install -y iptables-services

    # Forward packets between the private subnets and the internet, persisted across reboots.
    echo 'net.ipv4.ip_forward = 1' > /etc/sysctl.d/90-nat.conf
    sysctl --system

    # Masquerade everything from the VPC out of the primary interface. Save the live rules before enabling the
    # service so the package's default ruleset (which rejects forwarded traffic) is never loaded.
    IFACE=$(ip -o -4 route show to default | awk '{print $5}' | head -n 1)
    iptables -t nat -A POSTROUTING -o "$IFACE" -s ${local.vpc_cidr} -j MASQUERADE
    iptables-save > /etc/sysconfig/iptables
    systemctl enable iptables
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
