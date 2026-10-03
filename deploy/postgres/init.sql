-- Database-per-service: cada serviço tem o SEU banco e o SEU usuário.
-- O orders-service não tem credencial para ler o banco do users-service:
-- o único caminho até os dados de outro serviço é a API dele.
CREATE ROLE users_svc LOGIN PASSWORD 'users';
CREATE DATABASE users OWNER users_svc;

CREATE ROLE orders_svc LOGIN PASSWORD 'orders';
CREATE DATABASE orders OWNER orders_svc;
