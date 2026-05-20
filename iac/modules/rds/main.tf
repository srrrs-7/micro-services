# RDS Subnet Group
resource "aws_db_subnet_group" "microservices" {
  name       = "${var.environment}-microservices-db-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name        = "${var.environment}-microservices-db-subnet-group"
    Environment = var.environment
    Project     = "microservices"
  }
}

# Parameter Group for PostgreSQL
resource "aws_db_parameter_group" "postgres" {
  name   = "${var.environment}-microservices-postgres-param-group"
  family = "postgres15"
  description = "Custom parameter group for microservices PostgreSQL"

  parameter {
    name  = "max_connections"
    value = "200"
    apply_method = "pending-reboot"
  }

  tags = {
    Name        = "${var.environment}-microservices-postgres-param-group"
    Environment = var.environment
    Project     = "microservices"
  }
}

# Audit Database
resource "aws_db_instance" "audit" {
  identifier             = "${var.environment}-audit-db"
  engine                 = "postgres"
  engine_version         = "15.4"
  instance_class         = var.db_instance_class
  allocated_storage      = var.db_allocated_storage
  name                   = "audit"
  username               = var.db_username
  password               = var.db_password
  parameter_group_name   = aws_db_parameter_group.postgres.name
  vpc_security_group_ids = [var.rds_security_group_id]
  db_subnet_group_name   = aws_db_subnet_group.microservices.name
  skip_final_snapshot    = var.skip_final_snapshot
  deletion_protection    = var.deletion_protection
  storage_encrypted      = true
  backup_retention_period = var.backup_retention_period
  backup_window          = "03:00-04:00"
  maintenance_window     = "sun:04:00-sun:05:00"

  tags = {
    Name        = "${var.environment}-audit-db"
    Environment = var.environment
    Project     = "microservices"
    Service     = "audit"
  }
}

# Auth Database
resource "aws_db_instance" "auth" {
  identifier             = "${var.environment}-auth-db"
  engine                 = "postgres"
  engine_version         = "15.4"
  instance_class         = var.db_instance_class
  allocated_storage      = var.db_allocated_storage
  name                   = "auth"
  username               = var.db_username
  password               = var.db_password
  parameter_group_name   = aws_db_parameter_group.postgres.name
  vpc_security_group_ids = [var.rds_security_group_id]
  db_subnet_group_name   = aws_db_subnet_group.microservices.name
  skip_final_snapshot    = var.skip_final_snapshot
  deletion_protection    = var.deletion_protection
  storage_encrypted      = true
  backup_retention_period = var.backup_retention_period
  backup_window          = "04:00-05:00"
  maintenance_window     = "sun:05:00-sun:06:00"

  tags = {
    Name        = "${var.environment}-auth-db"
    Environment = var.environment
    Project     = "microservices"
    Service     = "auth"
  }
}