DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_app_user') THEN
    CREATE USER master_go_cqrs_app_user WITH PASSWORD 'master_go_cqrs_app_password';
  ELSE
    ALTER USER master_go_cqrs_app_user WITH PASSWORD 'master_go_cqrs_app_password';
  END IF;

  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_payment_user') THEN
    CREATE USER master_go_cqrs_payment_user WITH PASSWORD 'master_go_cqrs_payment_password';
  ELSE
    ALTER USER master_go_cqrs_payment_user WITH PASSWORD 'master_go_cqrs_payment_password';
  END IF;

  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_affiliate_user') THEN
    CREATE USER master_go_cqrs_affiliate_user WITH PASSWORD 'master_go_cqrs_affiliate_password';
  ELSE
    ALTER USER master_go_cqrs_affiliate_user WITH PASSWORD 'master_go_cqrs_affiliate_password';
  END IF;

  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_settlement_user') THEN
    CREATE USER master_go_cqrs_settlement_user WITH PASSWORD 'master_go_cqrs_settlement_password';
  ELSE
    ALTER USER master_go_cqrs_settlement_user WITH PASSWORD 'master_go_cqrs_settlement_password';
  END IF;

  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_agent_user') THEN
    CREATE USER master_go_cqrs_agent_user WITH PASSWORD 'master_go_cqrs_agent_password';
  ELSE
    ALTER USER master_go_cqrs_agent_user WITH PASSWORD 'master_go_cqrs_agent_password';
  END IF;

  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_orchestrator_user') THEN
    CREATE USER master_go_cqrs_orchestrator_user WITH PASSWORD 'master_go_cqrs_orchestrator_password';
  ELSE
    ALTER USER master_go_cqrs_orchestrator_user WITH PASSWORD 'master_go_cqrs_orchestrator_password';
  END IF;

  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'master_go_cqrs_keycloak_user') THEN
    CREATE USER master_go_cqrs_keycloak_user WITH PASSWORD 'master_go_cqrs_keycloak_password';
  ELSE
    ALTER USER master_go_cqrs_keycloak_user WITH PASSWORD 'master_go_cqrs_keycloak_password';
  END IF;
END
$$;

SELECT 'CREATE DATABASE master_go_cqrs_app OWNER master_go_cqrs_app_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_app')\gexec
SELECT 'CREATE DATABASE master_go_cqrs_payment OWNER master_go_cqrs_payment_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_payment')\gexec
SELECT 'CREATE DATABASE master_go_cqrs_affiliate OWNER master_go_cqrs_affiliate_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_affiliate')\gexec
SELECT 'CREATE DATABASE master_go_cqrs_settlement OWNER master_go_cqrs_settlement_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_settlement')\gexec
SELECT 'CREATE DATABASE master_go_cqrs_agent OWNER master_go_cqrs_agent_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_agent')\gexec
SELECT 'CREATE DATABASE master_go_cqrs_orchestrator OWNER master_go_cqrs_orchestrator_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_orchestrator')\gexec
SELECT 'CREATE DATABASE master_go_cqrs_keycloak OWNER master_go_cqrs_keycloak_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'master_go_cqrs_keycloak')\gexec
