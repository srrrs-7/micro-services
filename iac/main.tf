terraform {
  required_version = ">= 1.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# VPC for microservices
resource "aws_vpc" "microservices" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "${var.environment}-microservices-vpc"
    Environment = var.environment
    Project     = "microservices"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "microservices" {
  vpc_id = aws_vpc.microservices.id

  tags = {
    Name        = "${var.environment}-microservices-igw"
    Environment = var.environment
    Project     = "microservices"
  }
}

# Public Subnets
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.microservices.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 4, count.index)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name        = "${var.environment}-public-subnet-${count.index + 1}"
    Environment = var.environment
    Project     = "microservices"
    Tier        = "public"
  }
}

# Private Subnets for services
resource "aws_subnet" "private" {
  count                   = 2
  vpc_id                  = aws_vpc.microservices.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 4, 2 + count.index)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = false

  tags = {
    Name        = "${var.environment}-private-subnet-${count.index + 1}"
    Environment = var.environment
    Project     = "microservices"
    Tier        = "private"
  }
}

# Route Tables
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.microservices.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.microservices.id
  }

  tags = {
    Name        = "${var.environment}-public-route-table"
    Environment = var.environment
    Project     = "microservices"
  }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Private Route Table (for NAT Gateway later)
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.microservices.id

  tags = {
    Name        = "${var.environment}-private-route-table"
    Environment = var.environment
    Project     = "microservices"
  }
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# Availability Zones
data "aws_availability_zones" "available" {}

# Security Groups
resource "aws_security_group" "alb" {
  name        = "${var.environment}-alb-sg"
  description = "Allow HTTP inbound traffic"
  vpc_id      = aws_vpc.microservices.id

  ingress {
    description = "HTTP from VPC"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.environment}-alb-security-group"
    Environment = var.environment
    Project     = "microservices"
  }
}

resource "aws_security_group" "services" {
  name        = "${var.environment}-services-sg"
  description = "Allow traffic from ALB and internal communication"
  vpc_id      = aws_vpc.microservices.id

  ingress {
    description = "HTTP from ALB"
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  ingress {
    description = "PostgreSQL from services"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    security_groups = [aws_security_group.services.id]
  }

  ingress {
    description = "Redis from services"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    security_groups = [aws_security_group.services.id]
  }

  ingress {
    description = "gRPC from services"
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    security_groups = [aws_security_group.services.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.environment}-services-security-group"
    Environment = var.environment
    Project     = "microservices"
  }
}

resource "aws_security_group" "rds" {
  name        = "${var.environment}-rds-sg"
  description = "Allow PostgreSQL access from services"
  vpc_id      = aws_vpc.microservices.id

  ingress {
    description = "PostgreSQL from services"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    security_groups = [aws_security_group.services.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.environment}-rds-security-group"
    Environment = var.environment
    Project     = "microservices"
  }
}

resource "aws_security_group" "elasticache" {
  name        = "${var.environment}-elasticache-sg"
  description = "Allow Redis access from services"
  vpc_id      = aws_vpc.microservices.id

  ingress {
    description = "Redis from services"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    security_groups = [aws_security_group.services.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.environment}-elasticache-security-group"
    Environment = var.environment
    Project     = "microservices"
  }
}