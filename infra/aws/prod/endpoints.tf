# Free gateway endpoint: the api reaches S3 (media, and the ECR image layers) without going through the NAT instance.
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${var.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private_app.id, aws_route_table.private_db.id]

  tags = { Name = "${local.name}-s3" }
}
