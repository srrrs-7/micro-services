# ElastiCache Subnet Group
resource "aws_elasticache_subnet_group" "microservices" {
  name       = "${var.environment}-microservices-elasticache-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name        = "${var.environment}-microservices-elasticache-subnet-group"
    Environment = var.environment
    Project     = "microservices"
  }
}

# ElastiCache Security Group (already passed in as variable, but we can reference it here if needed)
# We are using the security group passed in from the root module.

# ElastiCache Redis Cluster
resource "aws_elasticache_replication_group" "microservices" {
  replication_group_id          = "${var.environment}-microservices-redis"
  description                   = "Redis cluster for microservices"
  engine                        = "redis"
  engine_version                = "7.0"
  node_type                     = var.node_type
  number_cache_clusters         = var.num_cache_clusters
  automatic_failover_enabled    = var.automatic_failover_enabled
  multi_zone_enabled            = var.multi_zone_enabled
  port                          = 6379
  subnet_group_name             = aws_elasticache_subnet_group.microservices.name
  security_group_ids            = [var.elasticache_security_group_id]
  maintenance_window            = "sun:05:00-sun:06:00"
  snapshot_retention_limit      = var.snapshot_retention_limit
  snapshot_window               = "04:00-05:00"
  apply_immediately             = true

  tags = {
    Name        = "${var.environment}-microservices-redis"
    Environment = var.environment
    Project     = "microservices"
    Service     = "redis"
  }
}

# ElastiCache Replication Group for read replicas (if num_cache_clusters > 1)
# The aws_elasticache_replication_group resource above already handles clustering when num_cache_clusters > 1
# and automatic_failover_enabled is true.

# Output the primary endpoint and reader endpoint for the replication group