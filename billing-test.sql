
-- CREATE DATABASE billing_test;
-- CREATE USER postgres WITH PASSWORD 'postgres';
-- public.user_balance definition
-- Drop table
-- DROP TABLE public.user_balance;
CREATE TABLE user_balance (
	user_id int4 NOT NULL,
	balance int4 NOT NULL DEFAULT 0,
	reserved_balance int4 NOT NULL DEFAULT 0,
	id serial4 NOT NULL,
	CONSTRAINT user_balance_pk PRIMARY KEY (id)
);

-- public."transaction" definition
-- Drop table
-- DROP TABLE public."transaction";
CREATE TABLE transactions (
	"type" varchar NOT NULL,
	user_id int4 NOT NULL,
	order_id int4 NULL,
	service_id int4 NULL,
	amount int4 NOT NULL,
	"timestamp" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
	id serial4 NOT NULL,
	CONSTRAINT transaction_pk PRIMARY KEY (id)
);