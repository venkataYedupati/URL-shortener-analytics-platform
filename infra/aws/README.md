# AWS ECS Deployment Notes

This folder contains lightweight production scaffolding for running the API, analytics worker, and dashboard on AWS ECS.

Expected managed dependencies:

- Amazon RDS for PostgreSQL.
- Amazon ElastiCache for Redis.
- Amazon MSK or a Kafka-compatible managed broker.
- Application Load Balancer forwarding dashboard and API traffic.
- CloudWatch Logs for API and worker containers.
- Application Auto Scaling policies for API tasks.

The JSON templates in this folder are intentionally parameterized with placeholders so they can be wired into Terraform, CDK, CloudFormation, or an existing platform pipeline.
