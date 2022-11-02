-- +goose Up
-- +goose StatementBegin
CREATE TABLE "subscription" (
  "id" uuid DEFAULT gen_random_uuid (),
  "customer_id" uuid NOT NULL,
  "plan_id" uuid NOT NULL,
  "start_date" timestamp(0) NOT NULL,
  "end_date" timestamp(0) NOT NULL,
  "state" varchar(64) NOT NULL,
  "stripe_id" varchar(255) NOT NULL,
  PRIMARY KEY ("id")
);

CREATE TABLE "subscription_card" (
  "id" uuid DEFAULT gen_random_uuid (),
  "customer_id" uuid NOT NULL,
  "stripe_id" varchar(255) NOT NULL,
  "last_digits" varchar(255) NOT NULL,
  "cardholder_name" varchar(255) NOT NULL,
  "status" varchar(255) NOT NULL,
  "date_created" timestamp(0) NOT NULL,
  "exp_month" int4 NOT NULL,
  "exp_year" int4 NOT NULL,
  "default" bool NOT NULL DEFAULT false,
  "address_city" varchar(255) NOT NULL DEFAULT ''::character varying,
  "address_country" varchar(255) NOT NULL DEFAULT ''::character varying,
  "address_line1" varchar(255) NOT NULL DEFAULT ''::character varying,
  "address_line2" varchar(255) NOT NULL DEFAULT ''::character varying,
  "address_state" varchar(255) NOT NULL DEFAULT ''::character varying,
  "address_zip" varchar(255) NOT NULL DEFAULT ''::character varying,
  PRIMARY KEY ("id")
);

CREATE TABLE "subscription_change_request" (
  "id" uuid DEFAULT gen_random_uuid (),
  "subscription_id" uuid NOT NULL,
  "plan_id" int4 NOT NULL,
  "date" timestamp(0) NOT NULL,
  PRIMARY KEY ("id")
);

CREATE TABLE "subscription_customer" (
  "user_id" uuid NOT NULL,
  "stripe_id" varchar(255) NOT NULL
);

CREATE TABLE "subscription_invoice" (
  "id" uuid DEFAULT gen_random_uuid (),
  "subscription_id" uuid NOT NULL,
  "plan_id" uuid NOT NULL,
  "stripe_id" varchar(255) NOT NULL,
  "url" varchar(255) NOT NULL,
  "status" varchar(255) NOT NULL,
  "date" timestamp(0) NOT NULL,
  "amount" int4 NOT NULL,
  "currency" varchar(255) NOT NULL,
  "period_start" timestamp(0) NOT NULL,
  "period_end" timestamp(0) NOT NULL,
  PRIMARY KEY ("id")
);

CREATE TABLE "subscription_plan" (
  "id" uuid DEFAULT gen_random_uuid (),
  "cost" int4 NOT NULL,
  "period" int4 NOT NULL,
  "title" varchar(255) NOT NULL,
  "stripe_id" varchar(255) NOT NULL,
  "allowed_sections" json,
  "benefits" json,
  "image" varchar(255) NOT NULL DEFAULT ''::character varying,
  "active" bool NOT NULL DEFAULT false,
  "trial" bool NOT NULL DEFAULT false,
  "trial_period" int4 NOT NULL DEFAULT 0,
  "limits_servers" int4 NOT NULL,
  "limits_users" int4 NOT NULL,
  "limits_companies" int4 NOT NULL,
  "limits_connections" int4 NOT NULL,
  "default" bool NOT NULL DEFAULT false,
  PRIMARY KEY ("id")
);

INSERT INTO "public"."subscription" ("id","customer_id","plan_id","start_date","end_date","state","stripe_id") VALUES 
('53088562-02f3-4403-852d-c488e0dfdfc4', '31efa308-42fa-453a-87c1-8a55a33e0943', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-05 16:48:43', '2021-10-07 16:48:43', 'trialing', 'sub_1JhGopExx8RtinWrrIMAK8uK'),
('9260d127-28ee-4b03-afa1-c5984008fe00', '31efa308-42fa-453a-87c1-8a55a33e0943', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-07 17:08:46', '2021-10-09 17:08:45', 'trialing', 'sub_1Ji05KExx8RtinWrfkNsjuwU'),
('9e803cc9-c0ad-4b31-98b2-9a50d05ffd1f', '61fc8b71-c0c3-41b0-85e1-77485f0483ad', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-08 12:55:03', '2021-10-10 12:55:03', 'trialing', 'sub_1JiIbLExx8RtinWrO28HuZW4'),
('a3cd3b12-acae-46ae-b5b5-bf4fe5d2905c', '2069e1a6-728a-4218-8373-ab9cc9c7f734', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-08 13:11:29', '2021-10-10 13:11:28', 'trialing', 'sub_1JiIrFExx8RtinWrjepJGJvG'),
('687d44ff-86f5-4310-a57d-e44e915e3709', '90a31861-1c82-4694-9016-85ee4f4ce2a5', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-08 13:28:00', '2021-10-10 13:27:59', 'trialing', 'sub_1JiJ7EExx8RtinWrARdcJ4Mx'),
('d01df544-c982-47f3-9d15-4f0433ed8ed2', 'd8f47009-0828-4055-b29a-45dbe10a16b2', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-08 14:59:38', '2021-10-10 14:59:37', 'trialing', 'sub_1JiKXuExx8RtinWr3bPWSEbR'),
('1c454790-93ce-48f3-a96a-2044b3eea7a9', '7e0d239b-efea-42eb-bfca-247fc18f03c8', '63bce164-9d48-4a42-816d-6faa84b2569a', '2021-10-08 15:01:12', '2021-10-10 15:01:12', 'trialing', 'sub_1JiKZRExx8RtinWrdWA4ZcDp'),
('c46f13ab-e1f7-48ea-9ff2-dc6d7c4dfd75', 'e1b94952-f569-486f-a123-aaf40d4250d6', '81bc2472-f737-4967-8256-e6dec688c5a7', '2021-10-08 15:29:55', '2021-10-10 15:29:55', 'trialing', 'sub_1JiL1DExx8RtinWrbYGGtEpI'),
('777cb894-f5e3-4160-beda-ef2f2b297948', 'e33fa68c-749d-4da5-9201-07f981d1cc52', '81bc2472-f737-4967-8256-e6dec688c5a7', '2021-10-09 09:40:33', '2021-10-11 09:40:33', 'trialing', 'sub_1Jic2fExx8RtinWr3idrNQcB');

INSERT INTO "public"."subscription_customer" ("user_id","stripe_id") VALUES 
('008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'cus_KJJIHcpq1myuJC'),
('31efa308-42fa-453a-87c1-8a55a33e0943', 'cus_KLymC6xAbTTUfb'),
('61fc8b71-c0c3-41b0-85e1-77485f0483ad', 'cus_KN2gYA0Lzqlk5m'),
('2069e1a6-728a-4218-8373-ab9cc9c7f734', 'cus_KN2xvoQneb2yvZ'),
('90a31861-1c82-4694-9016-85ee4f4ce2a5', 'cus_KN3DCBNNjAUnsq'),
('a43a184c-b33b-4cbf-b9a3-db1786b01cf1', 'cus_KN3M2Ij2y7VRX8'),
('d8f47009-0828-4055-b29a-45dbe10a16b2', 'cus_KN3gZWPsv3Upbh'),
('7e0d239b-efea-42eb-bfca-247fc18f03c8', 'cus_KN4i6M4uOsrmlR'),
('e1b94952-f569-486f-a123-aaf40d4250d6', 'cus_KN5BUpmtjt1v8O'),
('78e93832-05aa-467e-bc4c-b11ea94bc786', 'cus_KNMi0qSDyrhm4E'),
('e33fa68c-749d-4da5-9201-07f981d1cc52', 'cus_KNMmYzeKcpxJBV');

INSERT INTO "public"."subscription_plan" ("id","cost","period","title","stripe_id","allowed_sections","benefits","image","active","trial","trial_period","limits_servers","limits_users","limits_companies","limits_connections","default") VALUES 
('63bce164-9d48-4a42-816d-6faa84b2569a', 1000, 1, '$10 baks', 'plan_GK32fLo7OfcqsE', '["one","spider"]', '{"0":"servers","1":"members","2":"invites","3":"ldap","6":"log_activities"}', 'plans-project.svg', 't', 't', 2, 50, 5, 5, 5, 'f'),
('81bc2472-f737-4967-8256-e6dec688c5a7', 100, 1, '$1 baks', 'plan_EuP66qDevZxWwp', '["servers","members"]', '[]', 'plans-project.svg', 't', 't', 2, 5, -1, 5, -1, 't'),
('70dd72ef-3d78-4eb4-98fd-16a60862e8d5', 0, 1, '$0 baks', 'plan_EuP6gjMvEKH7Xu', '["servers","members","invites","log_activities"]', '{"1":"+ Garage Plan","2":"Up to 15 hosts","3":"Up to 30 members","4":"Up to 5 Companies"}', 'plans-project.svg', 't', 't', 2, -1, -1, -1, -1, 'f');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "public"."subscription";
DROP TABLE IF EXISTS "public"."subscription_card";
DROP TABLE IF EXISTS "public"."subscription_change_request";
DROP TABLE IF EXISTS "public"."subscription_customer";
DROP TABLE IF EXISTS "public"."subscription_invoice";
DROP TABLE IF EXISTS "public"."subscription_plan";
-- +goose StatementEnd
