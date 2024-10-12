-- +goose Up
-- +goose StatementBegin
INSERT INTO "profile" ("id", "alias", "name", "surname", "email", "password", "active", "confirmed", "role", "created_at") VALUES
('008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'admin', 'Penny', 'Hoyle', 'admin@werbot.net', '$2a$13$xXMJafmfthQDqVZSQ5HJ/u1EmQ8PqkVAGlwKrOWH.cOVZr2KfvSAK', true, true, 3, CURRENT_TIMESTAMP - INTERVAL '24 hour'),
('c180ad5c-0c65-4cee-8725-12931cb5abb3', 'user', 'Carly', 'Bender', 'user@werbot.net', '$2a$13$Wv2IkOgNUL6dNEw00U0GnuEzWrPSIgHdOgugnll5kFIYgLqrKpZOe', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '23 hour'),
('b3dc36e2-7f84-414b-b147-7ac850369518', 'user1', 'Harrison', 'Bowling', 'user1@werbot.net', '$2a$13$DJIAXdDTXli9vTbXbAoUl..Qs3Ns.B3CAtLzLvBb3fpHkPJxzOjxK', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '22 hour'),
('b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', 'user2', 'Terry', 'Henry', 'user2@werbot.net', '$2a$13$iiPNSGIY2x6O/xrDXeXHdObc0ubJPcOSVfiKMKGLreoUGcenVm.v2', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '21 hour'),
('68bf07a3-0132-4709-920b-5054f9eaa89a', 'user3', 'Carla', 'Snyder', 'user3@werbot.net', '$2a$13$s.Lwbx8YFHbGVA9b5EGyMerNjVJVrvAhmm33/PY5lWsivB86eQZke', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '20 hour'),
('395b237f-037e-49d4-b409-fd2f514242f6', 'user4', 'Clint', 'McLean', 'user4@werbot.net', '$2a$13$LBWcFi1.h8rg1WIoJCl4e.fzmCL7Buv7xfijaaNQLaR40YqAPACi6', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '19 hour'),
('8adff845-1354-4e59-8cd3-83f69fd193d3', 'user5', 'Chris', 'Burch', 'user5@werbot.net', '$2a$13$SfEnNd11n8k.b67PTpRvvO4jxXQzMhRbqv6dMoWv2XkCrbKQZc4Bq', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '18 hour'),
('9355d59b-430a-4904-b970-80b4b3a61677', 'user6', 'Justine', 'Wolfson', 'user6@werbot.net', '$2a$13$AvrM5ue0Fgg09EX/tcBEN./wMNXqmnB6cJyeydgiF/PyHPNrFLxBu', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '17 hour'),
('41570b56-9930-4e2b-8b24-a72232508cfd', 'user7', 'Lori', 'Henry', 'user7@werbot.net', '$2a$13$agZjEwUkh7lcMwGGHbMFi.FGNHigcYD0i7Rv/wdf3lAxw7UHWkmiy', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '16 hour'),
('c50fba0b-cd65-4bdd-b653-23bbc6a5c43b', 'user8', 'Kendrick', 'Hall', 'user8@werbot.net', '$2a$13$Zqa/F8Q8HHhjNF4s91ToyuDdteJt1rPBnsUHdwK0GtgXVwx4wAxmO', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '15 hour'),
('4de9448b-1024-470c-a6eb-593b59db21bb', 'user9', 'Brock', 'Solomon', 'user9@werbot.net', '$2a$13$r9po6fH8biLGLoDAMgWGyepxWtq13EriaxPMDmVpYnXSSVxyDJfI2', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '14 hour'),
('c30e1974-d6ac-4a9e-b091-456d3f98b24c', 'user10', 'Clinton', 'Proctor', 'user10@werbot.net', '$2a$13$tOfgFR0H09/85hpi9INdneFVFwI4rG2w9dJJr.Bq1Memvax4s8cy6', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '13 hour'),
('4cd386a5-9ad1-42cf-8a9c-7d2c03833467', 'user11', 'Noel', 'Terrell', 'user11@werbot.net', '$2a$13$Rm.4Z9QLyBulcU6ujWjjy.yhaNrE11EDSeJyyt34AZlgqkK6MntAS', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '12 hour'),
('b549bc5c-cbb5-4695-bd8f-87c61f6e84a1', 'user12', 'Zachariah', 'Woods', 'user12@werbot.net', '$2a$13$Lah47ZkB05TRUIQJaKQm4e/8JbKFkJOa8a2Lr8ZXwdVG/e.v/Pk/W', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '11 hour'),
('751b9b21-b397-4228-9f96-1e339374dfea', 'user13', 'Bonnie', 'Hoyle', 'user13@werbot.net', '$2a$13$IKaakp6JlUcMAq8fqZo3OOmphRmhIJv0g0URashWU22kb/LaQjngm', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '10 hour'),
('f975f12f-7b63-40df-a825-9817b821b7ab', 'user14', 'Allyson', 'Griffin', 'user14@werbot.net', '$2a$13$MiW4Pzw5IBfbGQLOwgzVuOZGbJ5d8.D0HvuIltcJDii8DAZBBdImi', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '9 hour'),
('2dcc93dd-88f0-4e52-bb5c-79a6425cf177', 'user15', 'Sadie', 'McClure', 'user15@werbot.net', '$2a$13$DkNj9fC383qby.SIyRe5/ufyEL1cgp.SePFiTkupUjgwJXV2ju.kO', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '8 hour'),
('dae49c41-6727-4925-803b-8cc4b1352c50', 'user16', 'Javon', 'Lloyd', 'user16@werbot.net', '$2a$13$0Le9wKh3QIwgryJBHAFNIuy1Rwt9QRfMVUaPW9dKFC3T1uciSYhm.', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '7 hour'),
('b889a039-616f-4002-a3fa-99f5c96c93a1', 'user17', 'Davion', 'Connolly', 'user17@werbot.net', '$2a$13$RmgPJg0qxclGIWkNQFKzauVV6KhKiS6bi3wSLaExMA.n6C62fmqga', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '6 hour'),
('465878d0-e80a-443c-ac26-d019cd2f0777', 'user18', 'Quinn', 'Baker', 'user18@werbot.net', '$2a$13$oTs9c8Vp5X9mHsesmwKiaulAfBcmpEKNwjpdG013DT74oNP.T9AZ.', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '5 hour'),
('4808e401-826e-453d-b5e6-6a618bb2fa90', 'user19', 'Abel', 'Griffin', 'user19@werbot.net', '$2a$13$P6Jp0YiZHpMOrCpYcBS7Au3gO1HhZQ7auqB7.u7RGRhfDxnqbWjtC', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '4 hour'),
('51c12bb6-2da6-491d-8003-b024f54a1491', 'user20', 'Kyleigh', 'Steele', 'user20@werbot.net', '$2a$13$VoyU1e6Zfmza9ytImcytGe64lT1mBKAVBKzAzmuQoUhUROrSG0CpO', true, true, 1, CURRENT_TIMESTAMP - INTERVAL '3 hour');

UPDATE "profile" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '48 hour' WHERE "id" = '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686';

INSERT INTO "profile_token" ("token", "profile_id", "action", "created_at") VALUES
('3c818d7c-72f3-4518-8eaa-755585192f21', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 5, CURRENT_TIMESTAMP - INTERVAL '5 hour'), -- for admin test
('55c9f79c-d827-43fc-8ad1-79e396d2432c', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 4, CURRENT_TIMESTAMP - INTERVAL '4 hour'),
('1b8bf7fe-d901-4c12-9c6c-e1689e45490f', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 5, CURRENT_TIMESTAMP - INTERVAL '1 hour'),
('07614765-81af-4e29-80e1-d6f01cc1b15a', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 5, CURRENT_TIMESTAMP - INTERVAL '1 day'), -- for user test
('5241b2ee-0417-4d14-a886-921f8fcda313', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 5, CURRENT_TIMESTAMP - INTERVAL '5 hour'),
('8c7c9b35-1c3e-4679-ab2d-3e176a2b73d9', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 5, CURRENT_TIMESTAMP - INTERVAL '5 hour'),
('0fcd88b3-8abb-4eb1-b96c-e0e49964cbca', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 5, CURRENT_TIMESTAMP - INTERVAL '10 hour'),
('88f2d90a-11da-43a3-8e79-f2d875593525', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 4, CURRENT_TIMESTAMP - INTERVAL '4 hour');

UPDATE "profile_token" SET "active" = false WHERE "token" = '0fcd88b3-8abb-4eb1-b96c-e0e49964cbca';

INSERT INTO "profile_public_key" ("id", "profile_id", "title", "key", "fingerprint") VALUES
('81f67bbb-79f8-47a1-863d-d9375f1626d3', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 1', 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCgIocHncbDnM3nLFmxt9TssvmhF5x5WMwng/QRA5jZEKYER93Jsh5inCk5C9K8rAubijeKSe7A/Pp88MSYq2nXQ3UafqXGmHvt++fvMwmJ4rgoxiQlT2GzLwSBhvqZHcPBLeeVwLQB9T6wYf/9/mptVHOrb/Wy/FH4g4j7v5DflazQMiW9SOiwlaDM7jJDEGgOdTrHaOyJycx5CGzUgW0ZOnLg8a0m1Fdjvv9lFq3ZQ2S5NqIXH4to+F7YsklHEwgjZawYj3ic10vN+ei4f7PuC28sNLKADayHHyhDdpmA3mKZTw2lIplk/uvE61ucmLbLynEMwdc6eh/64MWxgif/', '03:99:f1:4b:2d:d3:10:aa:65:74:cf:c8:d6:ec:48:e8'),
('22a98e3b-1da8-46f5-8962-b461685e7bd7', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 2', 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7BpymrfMLaou2l3LRIMjKhPZxDDZ6e09mqq+5yC8IEgjAtsoEh2i6H9IT6xHOHFqIe7thkRuDmNpAFni4aiaOREhL7U/+kdJ0trTDRv0WatxpcM7A0g4nR1aJPoxTBSG8No5JVLaaX5PNhZZx6Z4NOqr4TzhlZgoOj6CybzpZUgA+On4T2rN5BpbVfd1bbjUnpb80hNQVhShg150kMZKiH4vkqaT4CDDax3mvRETK9/hc2d3Cy3iyat/F/DncMAzSN9VdqjuOr7pFfomTKwduJ84akd2TlTdll+SyT4YIpokhemOrfXFWWxVKYVFn4aHmJ2QrovagHxLv98mL1VsP', 'af:d3:b3:aa:3c:3b:f9:58:5a:75:e9:76:37:0e:26:8a'),
('46d0d741-1d21-41ec-8b9b-2b456436ca48', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 3', 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDDZz0EtmfSphFWOMlvfsaa4dZVVhksjAUfqS1gfb4WWN75qaDy2CZ1VCtMd3C3uVteK/cVK94EqKhk3TRQjhw2PuuDYCcZsSyGuzc+lsSKkDgdRQwOzaVLCI8t0paRiCCsiahFSvuDt28YmCSIuVbWIX21gohXMunPY1zOaGz42tb1WFLtveHb2w3cZIYzqjumrxqQKTUzglyHoV5NiQJ5kSJ+OXLa4uZ6LVpoA9/vn2gcqZMvL22w43loItzRnxVMk/hsaZKTbqQtdCo+gUbMMblqeuiNLj5RwKW64iI8ErK0uFKdnpvQ7pzurvLBe70SKakDTEZmLOrsEMwk0hqX', 'a7:f3:b8:61:55:4f:c5:4d:91:72:0f:74:f0:42:00:fe'),
('a65abdf1-2f88-4dac-b818-1d4923c2b97e', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 'public_key 4', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGWl2aY8FicEWNAlrQ+DwmhonSuhU8SsXJErdO9WpPKN', 'cc:11:5d:dc:36:91:f3:b3:10:97:48:e4:ef:08:94:d1'),
('bb9c2779-0b1a-4dcf-b79e-0f486d9c43d6', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 'public_key 5', 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCcqfLcl6onqLKxHouuHFAQIN22Ujs+nh0MYP8TqiXvzEfXel8H+SPPi1x1wwCPwUC6KcTeWzhRB1DNzvvGuWbJzxl5uDErF0bZ4sj9yKBVYQ2exKErZX9rj04zCPU8BwRU1G/tsI816RB0NrPkw5b53IKaGr9zQsI3KQrKrCFe7Huq/PytbDe0PYYHYbUytO/ZHGVCnwz2/TI5LcksU2H3uWkTQUlIhdkAEDB4vZueLOVypinIG8ez2jFhieHpwWzVcHf8b3a+XjRflWsB3diBliVfJ0vKcewDaj8FSE4WZ+XVzTUMrv/GtHyV9M+SXVj3Ren7+z5DZ5ETTSJHvB8f', '59:e3:58:4b:a8:2a:49:43:d5:5a:fd:c6:58:ff:af:8a'),
('e14acff0-d1d9-4e4e-8b7e-92b224c35e51', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 9', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHaUvLp9R83NC1uO4092tl1eQUvplXcYAP+H0CjX9xLp', 'fe:3e:9e:9c:c8:8a:c8:04:93:ca:3e:4e:08:99:da:09'),
('cb722ae3-203c-4dd0-9383-f7f5b70e9217', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 8', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICORhW35zrW+hdDXKE7jLco0uPjnEDcttthtcEpgCDZ2', '0f:5f:3a:1a:da:eb:6f:0a:c4:df:9c:11:56:9f:36:e1'),
('852b684c-4c14-44da-a764-1b92c5f7e189', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 7', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDW5ExclPxXvynfI3cw2iomQA1x195UINPV8+Q7ojPlm', '9d:06:14:58:67:7e:ee:97:6f:d9:12:65:be:37:e9:70'),
('9a7919a0-3308-4ac3-957e-6919870e3568', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 6', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHygIeEGZSXy6WVav1mg4+fc2rHYhWULNwn7yL16pif8', '5a:ac:ee:fb:89:a3:15:16:70:3b:25:85:c9:f6:9f:c6'),
('1c87762f-2d0f-4b92-a636-52fef4ff53a5', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 14', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMKIhlysVKLIuxl2RvfpdGptcvghhSkWfmmum1pSVze0', 'b6:07:6a:ef:82:e3:73:47:56:69:3f:3d:c7:d7:6f:23'),
('5c04dab5-9099-4718-893b-88d58d7763c8', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 13', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJsAA9/mgf5oS0NixfsayagA+Hk7X1xJRbmCJnbSAI9e', '9e:39:7e:69:b8:82:31:f3:a6:3d:7a:0f:bc:ec:1a:bf'),
('85a70dba-696f-44fc-8669-11d1b937659a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 12', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHQYSGBNj5m2AVgt26VBrptwfgSC4gSvEoLdAQgluyQK', 'a1:2b:6c:10:fd:36:10:f1:06:2a:6d:e9:cf:4e:68:f0'),
('81033c2b-a69f-4290-b26a-346d28977e6e', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 11', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJIII56btowQHAD+UpiO/LOI2nf1kuGiodSO8GHUcvPX', 'c0:69:05:53:b8:b0:90:22:f8:e8:04:56:92:f5:b2:01'),
('74db70d8-0597-4841-aebc-57f579341ff8', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'public_key 10', 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINiF7+RDhS9QzKZxVQj0ElpgQzyupViVyyHMHWH+ZgMD', 'fc:58:a8:48:d1:9f:2d:4e:38:4d:3e:ea:fc:f6:bd:a4');

UPDATE "profile_public_key" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '5 hour', "locked_at" = CURRENT_TIMESTAMP WHERE "id" = 'a65abdf1-2f88-4dac-b818-1d4923c2b97e';

INSERT INTO "project" ("id", "owner_id", "title", "alias") VALUES
('2bef1080-cd6e-49e5-8042-1224cf6a3da9', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project1', 'Y93iyI'), -- admin
('fe52ca9b-5599-4bb6-818b-1896d56e9aa2', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project2', 'U2g7uG'), -- admin
('e29568e6-c8f8-4555-a531-58a44554046f', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project9', 'u6pU7P'), -- admin
('dab9c84c-9b8d-4737-ba21-ae8d2ea73d10', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project8', 'MmHh42'), -- admin
('26060c68-5a06-4a57-b87a-be0f1e787157', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project7', '68rwRW'), -- admin
('926e432f-36df-433b-886d-f9a5b60705ad', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project6', 'U27uHh'), -- admin
('cb507c82-a368-4cd9-9030-557ccbd8761e', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project5', 'joO35J'), -- admin
('c22bc277-36c0-4a90-86e5-44b2f607046e', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project4', 'l4Ss7L'), -- admin
('ca7e65a4-76ea-4802-9f4f-3518a3416985', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project10', 'X8xf2F'), -- admin
('af0b3b3f-3ba1-4ad6-8f47-184e861af1f0', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project12', '2yY9Ff'), -- admin
('cb06da23-91d5-482a-b082-86b7b4b1df8a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', 'project11', 'nQq65N'), -- admin
--
('d958ee44-a960-420e-9bbf-c7a35084c4aa', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 'project3', 'C8cXx0'), -- user
('f85c4597-005d-48a6-b643-a21adf19a4aa', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 'project12', 'kAa40K'), -- user
('83d401e4-fda4-404e-8c2a-da58b03919c1', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', 'project13', '9Yyz9Z'), -- user
--
('09576675-54f0-4515-abdc-b5a39ad80b01', 'b3dc36e2-7f84-414b-b147-7ac850369518', 'project14', '79YysS'), -- user 1
('f32803b2-f4ab-491a-a463-11d89bf474c8', 'b3dc36e2-7f84-414b-b147-7ac850369518', 'project15', 'jqQ3J6'), -- user 1
('61ec6f92-61e6-482e-a9d5-f485a46a9515', 'b3dc36e2-7f84-414b-b147-7ac850369518', 'project16', 'p5Pp6P'), -- user 1
--
('c35e9da1-605a-41e1-af74-5e08875023ab', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', 'project17', 'D9zZ1d'), -- user 2
('a6677b05-1fdd-4c29-b1fd-48be1d41cb50', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', 'project18', 'CF2fc1'), -- user 2
('213ebb83-d8b4-4677-8acd-9bb8855a400f', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', 'project19', '7S6sqQ'), -- user 2
--
('4de5ed75-f0d8-4f9b-b1d5-1f089e309cd7', '68bf07a3-0132-4709-920b-5054f9eaa89a', 'project20', '3x9iXI'), -- user 3
('a91124e5-79c7-40f9-be04-a4519927ef47', '68bf07a3-0132-4709-920b-5054f9eaa89a', 'project21', 'R36irI'), -- user 3
('3093c300-c0dc-4525-85fb-77b34653d55f', '68bf07a3-0132-4709-920b-5054f9eaa89a', 'project22', 'Bm0M4b'), -- user 3
('419b45a0-a94f-461e-b6a3-8bca3d637f7d', '68bf07a3-0132-4709-920b-5054f9eaa89a', 'project23', 'Kk4e1E'), -- user 3
--
('18bc1d61-7c16-488b-b0b8-0c5b0679f41d', '395b237f-037e-49d4-b409-fd2f514242f6', 'project24', '7nN5Ss'), -- user 4
('d8471f69-b6bf-4110-ab5c-305fc813aefb', '395b237f-037e-49d4-b409-fd2f514242f6', 'project25', 'i3I6rR'), -- user 4
--
('2b19d6b5-d9e3-4833-af2c-23612c3b98da', '8adff845-1354-4e59-8cd3-83f69fd193d3', 'project26', 'DBd0b1'), -- user 5
('45e2673b-646b-4887-845d-b3daf10fac81', '8adff845-1354-4e59-8cd3-83f69fd193d3', 'project27', '3x9IiX'), -- user 5
('db87e46d-8daa-4ac7-93d2-5eb5b8870a47', '8adff845-1354-4e59-8cd3-83f69fd193d3', 'project28', '1C8wcW'), -- user 5
--
('0826d9e7-ac3b-4149-8fb0-a06fd2550ad5', '9355d59b-430a-4904-b970-80b4b3a61677', 'project29', 'm47sSM'), -- user 6
('c0ebc502-3759-41e5-9960-49a7b0895ca5', '9355d59b-430a-4904-b970-80b4b3a61677', 'project30', 'u6UR7r'), -- user 6
('94df7900-8ca4-4916-9dfd-ef37c8d0ddb8', '9355d59b-430a-4904-b970-80b4b3a61677', 'project31', '3y9kYK'), -- user 6
--
('5d7e1e1a-07f1-45e6-9f96-682b3cb25acc', '41570b56-9930-4e2b-8b24-a72232508cfd', 'project32', '7pPs5S'), -- user 7
('a7447176-8365-4251-bc35-c2be708730c8', '41570b56-9930-4e2b-8b24-a72232508cfd', 'project33', 'Cm1Mc4'), -- user 7
('c502a4df-1844-44fb-99f7-b3e0084c4f95', '41570b56-9930-4e2b-8b24-a72232508cfd', 'project34', '0Cc9yY'), -- user 7
--
('20f1696f-a203-4651-a54d-bb0c8514b38b', 'c50fba0b-cd65-4bdd-b653-23bbc6a5c43b', 'project35', 'DdI3i1'), -- user 8
('c170334b-9d6c-4d0c-842f-df3543048ce4', 'c50fba0b-cd65-4bdd-b653-23bbc6a5c43b', 'project36', 'l47sLS'), -- user 8
--
('c76f8a65-246b-4c80-9c1d-29e3347680bf', '4de9448b-1024-470c-a6eb-593b59db21bb', 'project37', '69RYyr'), -- user 9
('71331ca4-975c-40de-9197-08cd00a39194', '4de9448b-1024-470c-a6eb-593b59db21bb', 'project38', 'AA00aa'), -- user 9
('e3f2319c-f972-42c6-8b53-57d9cc668ad4', '4de9448b-1024-470c-a6eb-593b59db21bb', 'project39', 'mA04Ma'), -- user 9
('679d428e-48cc-40ae-9576-c759eab0d0e7', '4de9448b-1024-470c-a6eb-593b59db21bb', 'project40', 'joJO53'), -- user 9
--
('429287d2-ee96-47dd-b003-f4e4563f2485', 'c30e1974-d6ac-4a9e-b091-456d3f98b24c', 'project41', '8vVf2F'), -- user 10
('272154e9-9790-4809-b7a7-ce06e76e6c88', 'c30e1974-d6ac-4a9e-b091-456d3f98b24c', 'project42', 'A0a6Pp'), -- user 10
('de3998e3-4d71-4bc0-ba98-bbbaf2021af3', 'c30e1974-d6ac-4a9e-b091-456d3f98b24c', 'project43', 'pR56Pr'), -- user 10
--
('f1b39670-741c-4326-b337-bd75ea8dd314', '4cd386a5-9ad1-42cf-8a9c-7d2c03833467', 'project44', 'EDd1e1'), -- user 11
('3042ce7b-d600-4154-af73-dc76e2c3ade0', '4cd386a5-9ad1-42cf-8a9c-7d2c03833467', 'project45', 'FI3if1'), -- user 11
--
('bcd75eee-ad8f-4000-a9bd-c761793fb0a1', 'b549bc5c-cbb5-4695-bd8f-87c61f6e84a1', 'project46', 'kK2F3f'), -- user 12
('3cd8cfcb-3efb-4b1b-8a0e-2ab5592a2eb8', 'b549bc5c-cbb5-4695-bd8f-87c61f6e84a1', 'project47', 'jpPJ36'), -- user 12
('fafd3ed5-276d-4af1-a610-c0db2eeb2668', 'b549bc5c-cbb5-4695-bd8f-87c61f6e84a1', 'project48', 'CF20cf'), -- user 12
('f1fdad1f-f2f2-4ef8-8978-62f5b33a6071', 'b549bc5c-cbb5-4695-bd8f-87c61f6e84a1', 'project49', 'R2rG6g'), -- user 12
('e795b33c-bcdc-4587-abcd-3dc857606568', 'b549bc5c-cbb5-4695-bd8f-87c61f6e84a1', 'project50', 'jqQJ63'); -- user 12

UPDATE "project" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '24 hour', "locked_at" = CURRENT_TIMESTAMP  - INTERVAL '3 hour' WHERE "id" = 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2';
UPDATE "project" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '22 hour', "locked_at" = CURRENT_TIMESTAMP  - INTERVAL '1 hour' WHERE "id" = 'f1fdad1f-f2f2-4ef8-8978-62f5b33a6071';
UPDATE "project" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '23 hour', "locked_at" = CURRENT_TIMESTAMP  - INTERVAL '2 hour', "archived_at" = CURRENT_TIMESTAMP WHERE "id" = '83d401e4-fda4-404e-8c2a-da58b03919c1'; -- archived project
UPDATE "project" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '22 hour', "locked_at" = CURRENT_TIMESTAMP  - INTERVAL '1 hour', "archived_at" = CURRENT_TIMESTAMP WHERE "id" = 'e795b33c-bcdc-4587-abcd-3dc857606568'; -- archived project

INSERT INTO "project_api" ("id", "project_id", "api_secret", "api_key", "active") VALUES
('a1059d92-d032-427c-9444-571967d1f9a5', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'AweHj3rtANGfy0gG021ptsDzYMwYmgwnY11CC', '3GZBSPqDi7r1FDzYMUNV41l9HOJlb9y8b3ZI9', true),
('5d9e8b3e-d222-426e-980f-4664e52308c8', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'U8uSEKjM5a979LIQ2EhEUxKcP0OGYUTLBDzYM', 'r8tKBI6C64LDzYM2Rqxi1fZ8zMyikOERwMv0g', true),
('268def26-4efb-47c4-b699-f34903cf05f5', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'aDzYMy9g3mmsq3XazPLvvCbj4kJAsgatxBDVW', '5tYJOkr3oLCOEvhw3nB83AmDzYM7yJsJ0Sonl', true),
('974c8001-4f8a-4ccc-b9db-9cf29e1405b2', 'e29568e6-c8f8-4555-a531-58a44554046f', 'aUAlDzYMEnu44p6FMj4ID6RSAjBwo9XAMXMfe', 'WSWrmtLpsurpX1BYacO4DzYM2eswFoZf20lm9', true),
('21952d70-6ecc-4ec1-8ae1-46d27ea22b14', 'dab9c84c-9b8d-4737-ba21-ae8d2ea73d10', 'LInFiquDzYMekJ35P8BL4LZUfeaBbMJoAqWWN', 'AMXFfDc0qhyvf3zRDz5YMNDzYMLeQRIpoAcer', true),
('e4c33b9c-e307-4499-8300-30994027fa2a', '26060c68-5a06-4a57-b87a-be0f1e787157', 'bnMCqHjd3UenDz7YMi817gix73T8CUGBbMt8c', 'qq0UDzYMzJv46BYZAEmHfPqUreCQmDQA1nkoA', true),
('d2ea591a-1090-4fd2-91aa-19062befa5b4', '926e432f-36df-433b-886d-f9a5b60705ad', 'k52971tpi0DKE6rdhDzYMMohT7eNpdGujGUo8', 'y0TkiLHb0tBRZuh7XyvOi81jT2JBo9DzYM3s', true),
('3949db18-d7cd-49a6-98c4-6631f63665d2', 'cb507c82-a368-4cd9-9030-557ccbd8761e', 'tkp7ggd4INXCl92PcozURDzYMwURIXxC6U8OE', 'DzYMa7OMXw2yGNK59zrrLTz8aqruJvqVdZNOf', true),
('938a5665-06e1-4989-b850-5dbcd3703118', 'c22bc277-36c0-4a90-86e5-44b2f607046e', 'SDzMr0OPVhU8G1P6ff8Ub0go1DzYMQFoRo1nA', 'Gqdr0s8jpSilaDzYM166N8sxkOqj9EhAgiSoE', true),
('d2fa410c-749a-41ee-ba8c-c38c9c115d3b', 'ca7e65a4-76ea-4802-9f4f-3518a3416985', 'Pi4kEhZ7Tl1ehdYigafI8e3hDzYM0jawVHqVd', 'hhyeLotT7iZ4UwGGO0bRDzYMP7stwKrIc5vc7', true),
('77d2c421-1ad6-40fe-8ec7-634066aa244b', 'af0b3b3f-3ba1-4ad6-8f47-184e861af1f0', 'ulFRsDzYMR19KLt1gceq5utTeI4To9R4FUe8D', 'WrG6nflBDzYMEZ1dQa9xlG4gCQOqIPei9iciP', true),
('81ae0bbe-b746-44a6-a1f4-8467ff17bf5e', 'f85c4597-005d-48a6-b643-a21adf19a4aa', 'B2KfVCcE8tbVDeF317D11dvbvOB00o5d0kT08', 'PBhcCYC2ccIcpeC00B06H191b300yP5bpC0Ei', true),
('c9ae4704-3e86-4385-8b49-23c012d3b8d5', 'cb06da23-91d5-482a-b082-86b7b4b1df8a', '5Gm8vYc5sxpO7DzYMR5xhGzg29W9cYgAShkTF', 'POyx1k1H6YuEAim2hDzYMCU1BvMoBeg9c8KMP', true);

INSERT INTO "project_invite" ("token", "project_id", "name", "surname", "email", "status", "ldap_user", "ldap_name") VALUES
('9e72fc1e-30c1-4f23-b4ac-88df12351aac', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'admin', 'admin1', 'admin@werbot.net', 2, false, NULL),
('15348c8a-894e-49e6-88a9-79aa058892f3', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'user', 'test1', 'user@werbot.net', 1, false, NULL),
('3ec414fc-0bbe-44b8-b1d2-ff99171b4963', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'use99', 'test99', 'invite99@werbot.net', 1, false, NULL),
--
('b5b128d7-ff0c-479c-8216-14eedf9265ad', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'user', 'test1', 'user@werbot.net', 2, false, NULL),
--
('37aec639-dd1c-4c73-a8e7-add2016050f7', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'user', 'test1', 'user@werbot.net', 2, false, NULL),
('a60b7092-660e-4d05-a5d2-7d9b23b276b1', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'test3', 'user3', 'user3@werbot.net', 1, false, NULL),
('04439feb-f981-4581-8f2c-96f21418f258', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite100', 'user100', 'invite100@werbot.net', 1, false, NULL),
('4f405197-f312-4acb-a333-3781b6d23f9f', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite101', 'user101', 'invite101@werbot.net', 1, false, NULL),
('b49b830b-d0be-4757-a5ce-ec2677735cc5', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite102', 'user102', 'invite102@werbot.net', 1, false, NULL),
('b44dc303-5857-4bf6-af24-db64f3be1540', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite103', 'user103', 'invite103@werbot.net', 1, false, NULL),
('13614994-63d4-4e1d-9ccd-d80bf2356910', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite104', 'user104', 'invite104@werbot.net', 1, false, NULL),
('f0e9381e-ed3c-431c-bca2-019f8712436f', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite105', 'user105', 'invite105@werbot.net', 1, false, NULL),
('dc1dc751-1cb9-43af-a4e7-30c1100909cb', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite106', 'user106', 'invite106@werbot.net', 1, false, NULL),
('1b53f991-fa1b-401b-8ff4-1fe2f2498fe6', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite107', 'user107', 'invite107@werbot.net', 1, false, NULL),
('1e6fdbf6-beb3-4a0e-93f6-3fa033fc2626', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite108', 'user108', 'invite108@werbot.net', 1, false, NULL),
('4d7f9bee-94e7-4463-a62d-0b939efe6096', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite109', 'user109', 'invite109@werbot.net', 1, false, NULL),
('506732f3-8477-4963-af08-93e5071b381c', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'invite110', 'user110', 'invite110@werbot.net', 1, false, NULL);

INSERT INTO "project_member" ("id", "project_id", "profile_id", "active", "online", "role") VALUES
('9d3f7efc-14a5-436d-a763-314441d6e0a5', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', true, false, 1), -- admin project1, user member
('92de7a44-08fc-4d42-aab5-37f86fd598a2', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'b3dc36e2-7f84-414b-b147-7ac850369518', true, true, 1), -- admin project1, user1 member
('bac61932-ac1d-4f3a-b842-17b136bd1346', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', true, true, 1), -- admin project1, user2 member
('455b8913-c71d-4536-9b03-70bcb487b7cb', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', '68bf07a3-0132-4709-920b-5054f9eaa89a', false, false, 1), -- admin project1, user3 member
('4df3d0d2-560a-4583-b129-13cac558df2f', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', '395b237f-037e-49d4-b409-fd2f514242f6', true, true, 1), -- admin project1, user4 member
('c985aa3e-3056-42e8-9bb5-538fe75fea3e', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', '8adff845-1354-4e59-8cd3-83f69fd193d3', true, false, 1), -- admin project1, user5 member
('e729dcdc-8e95-4af8-bc90-d703b35ca4a3', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', '9355d59b-430a-4904-b970-80b4b3a61677', false, false, 1), -- admin project1, user6 member
('c66c20e4-45ab-4442-9bd9-5e3ceef2df0c', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', true, true, 1), -- admin project2, user member
('fd56eb85-7f40-40b5-b891-218edbca9f10', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'b3dc36e2-7f84-414b-b147-7ac850369518', false, false, 1), -- admin project2, user1 member
('5797fe00-4189-4c32-967e-7a33b5d1c067', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', false, false, 1), -- admin project2, user2 member
('3776ab61-ec15-4184-b790-66169755ed80', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', '68bf07a3-0132-4709-920b-5054f9eaa89a', true, false, 1), -- admin project2, user3 member
('8f2e0c85-fe63-4db6-a7d1-e39b6029b94d', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', '395b237f-037e-49d4-b409-fd2f514242f6', false, false, 1), -- admin project2, user4 member
('0f66a246-4752-4529-b3d9-4da662a5c168', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', '8adff845-1354-4e59-8cd3-83f69fd193d3', true, true, 1), -- admin project2, user5 member
('bae967a2-3981-4a84-8fe8-eb2ec8393e8d', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', '9355d59b-430a-4904-b970-80b4b3a61677', true, true, 1), -- admin project2, user6 member
('bafe193e-0782-481c-bdcf-22348a921f17', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', '41570b56-9930-4e2b-8b24-a72232508cfd', false, false, 1), -- admin project2, user7 member
('60897721-cdb0-4f72-b161-95e0bedaff13', 'c22bc277-36c0-4a90-86e5-44b2f607046e', 'c180ad5c-0c65-4cee-8725-12931cb5abb3', true, true, 1), -- admin project4, user member
('57151ddb-a03b-4c83-b934-eacfa17cad89', 'c22bc277-36c0-4a90-86e5-44b2f607046e', 'b3dc36e2-7f84-414b-b147-7ac850369518', false, false, 1), -- admin project4, user1 member
('d4b33db8-1074-425b-aa45-cd2c69fa66ce', 'c22bc277-36c0-4a90-86e5-44b2f607046e', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', true, true, 1), -- admin project4, user2 member
('12b9df2d-d3da-44fd-a974-e9ef54970524', 'c22bc277-36c0-4a90-86e5-44b2f607046e', '68bf07a3-0132-4709-920b-5054f9eaa89a', true, false, 1), -- admin project4, user3 member
('5ffd86c8-c74a-4a6b-af73-53e53faf2070', 'c22bc277-36c0-4a90-86e5-44b2f607046e', '395b237f-037e-49d4-b409-fd2f514242f6', true, true, 1), -- admin project4, user4 member
('de602a3a-fe6a-406c-892e-3e7f60040a9e', 'c22bc277-36c0-4a90-86e5-44b2f607046e', '8adff845-1354-4e59-8cd3-83f69fd193d3', false, false, 1), -- admin project4, user5 member
('407687f2-60b9-4edb-8ec4-14e13db0be51', 'c22bc277-36c0-4a90-86e5-44b2f607046e', '9355d59b-430a-4904-b970-80b4b3a61677', true, true, 1), -- admin project4, user6 member
--
('4fc69519-b683-46f0-860c-3e7f12a17563', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', true, false, 1), -- user project3, admin member
('49a10a09-0bb3-48af-99cb-181533692585', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'b3dc36e2-7f84-414b-b147-7ac850369518', true, true, 1), -- user project3, user1 member
('43ab80dd-ffe6-4881-aa8d-52b56ea715d2', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0', false, true, 1),  -- user project3, user2 member
('56306870-184b-4e38-8f46-f835952263ec', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '68bf07a3-0132-4709-920b-5054f9eaa89a', true, false, 1),  -- user project3, user3 member
('96cc03ef-35b1-47f4-afb6-5e57e829e493', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '395b237f-037e-49d4-b409-fd2f514242f6', true, false, 1),  -- user project3, user4 member
('cf1a7752-a1ea-4174-93c8-f90bebf7adcf', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '8adff845-1354-4e59-8cd3-83f69fd193d3', true, true, 1),  -- user project3, user5 member
('a7eb3ed1-2c40-4bd6-909c-2e9175af68a6', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '9355d59b-430a-4904-b970-80b4b3a61677', true, false, 1),  -- user project3, user6 member
('84d75ce2-d3f5-49f6-bac4-e2361f5899ae', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '41570b56-9930-4e2b-8b24-a72232508cfd', true, true, 1),  -- user project3, user7 member
('545f8d96-c77d-4b63-a2e6-a53bdb9d443c', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'c50fba0b-cd65-4bdd-b653-23bbc6a5c43b', true, true, 1),  -- user project3, user8 member
('7f717b66-34b5-4707-b9b8-0f63e8e034de', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '4de9448b-1024-470c-a6eb-593b59db21bb', true, true, 1),  -- user project3, user9 member
('de040931-6977-4629-aab0-6d621ff368a3', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'c30e1974-d6ac-4a9e-b091-456d3f98b24c', true, true, 1),  -- user project3, user10 member
('e7492145-3b8e-4d06-a3a7-81fb6deb3571', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', '4cd386a5-9ad1-42cf-8a9c-7d2c03833467', true, true, 1);  -- user project3, user11 member

UPDATE "project_member" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '5 hour' WHERE "id" = '9d3f7efc-14a5-436d-a763-314441d6e0a5';
UPDATE "project_member" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '5 hour', "locked_at" = CURRENT_TIMESTAMP - INTERVAL '2 hour' WHERE "id" = 'c66c20e4-45ab-4442-9bd9-5e3ceef2df0c';
UPDATE "project_member" SET "created_at" = CURRENT_TIMESTAMP - INTERVAL '2 hour', "locked_at" = CURRENT_TIMESTAMP - INTERVAL '1 hour' WHERE "id" = '43ab80dd-ffe6-4881-aa8d-52b56ea715d2';

INSERT INTO "scheme" ("id", "project_id", "title", "description", "active", "audit", "online", "scheme_type", "access") VALUES
-- servers:
-- tcp
('f5fb2d7a-578d-484d-bda7-301c939f5268', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'TCP connect #1', '', true, false, false, 101, '{"alias":"tNhU7z","address":"192.168.1.1","port":200}'),
('d9593b0c-d494-403e-8643-76e3946aca57', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'TCP connect #2', '', true, false, false, 101, '{"alias":"hd5fUr","address":"192.168.1.2","port":210}'),
('d0fa7301-dffa-444f-aca7-0124a3e8a17c', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'TCP connect #3', '', true, false, false, 101, '{"alias":"r9pcU3","address":"192.168.1.3","port":220}'),
('beb17acc-7758-4628-81c8-3f9121962037', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'TCP connect #1', '', true, false, false, 101, '{"alias":"NQZ3zp","address":"192.168.1.4","port":230}'),
('949a2f0f-1e17-4e51-9c0c-c84216554911', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'TCP connect #4', '', true, false, false, 101, '{"alias":"Cipfq8","address":"192.168.1.5","port":240}'),
('d3128cea-0359-4104-bed1-d9497b174f31', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'TCP connect #5', '', true, false, false, 101, '{"alias":"nLmMSt","address":"192.168.1.6","port":250}'),
-- udp
('c34f1c81-3a49-442d-8741-c8232c4590b1', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'UDP connect #1', '', true, false, false, 102, '{"alias":"07wxMg","address":"192.168.2.1","port":502}'),
('4e743d21-da72-49bb-ae25-66a463986e45', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'UDP connect #2', '', true, false, false, 102, '{"alias":"xV42z5","address":"192.168.2.2","port":503}'),
('6fdc0d40-5055-4483-9045-e925f338751e', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'UDP connect #3', '', true, false, false, 102, '{"alias":"Sx7yXv","address":"192.168.2.3","port":504}'),
('73f3d6cd-792c-410d-888f-e621a78b049e', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'UDP connect #1', '', true, false, false, 102, '{"alias":"4bXvzg","address":"192.168.2.4","port":505}'),
('6a77b657-cf64-44d2-b399-a28fd7d7c603', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'UDP connect #4', '', true, false, false, 102, '{"alias":"e1B4tV","address":"192.168.2.5","port":506}'),
('42c750eb-a12e-4b31-9014-06c0305a60b6', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'UDP connect #5', '', true, false, false, 102, '{"alias":"q9Ec2R","address":"192.168.2.6","port":507}'),
-- ssh
('0c3a8869-6fc0-4666-bf60-15475473392a', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Develop server #2', '', true, false, false, 103, '{"alias":"7Ub2Gn","port":2211,"key":{"login":"ubuntu11","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB1PFvdQd6ezh0gsz33IFVoK9Hn0TUqLTtCalrN0hyuO werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAACmFlczI1Ni1jdHIAAAAGYmNyeXB0AAAAGAAAABBJqQ/pVB\n2VjG/NBg5cdcxKAAAAEAAAAAEAAAAzAAAAC3NzaC1lZDI1NTE5AAAAIB1PFvdQd6ezh0gs\nz33IFVoK9Hn0TUqLTtCalrN0hyuOAAAAkFj/WUfoF+FWPbyTgEw2YadKUYAi1eVkJutLHZ\nWTNygXWoifZeKys78H23MPfS9AlmvV3JxsI+x53o3LOyr+DQdzC7o85fRuVVuZalL1JfyF\nviPqRIWSZWhnZtuyZP09T1EkmYZ+Dnf2CGCi75DicQvSIw6AJqJZAYmLtQ4qqqo2edVEHW\nQlvtMIlY1TJ3Q6aw==\n-----END OPENSSH PRIVATE KEY-----", "passphrase": "test"}}}'),
('156d8d65-cfe5-48a4-a636-198a5f509abf', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Git server', '', false, false, true, 103, '{"alias":"onxzU5","address":"secret.net.google.com","port":2206,"password":{"login":"ubuntu6","password":"ubuntu6"}}'),
('3e52d691-4432-4906-891d-f3816112f6cc', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Cache server', '', true, false, false, 103, '{"alias":"xceZEi","address":"127.0.0.2","port":2200,"key":{"login":"ubuntu","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFXO9/AMMVwrqKMw980tkk/LuCrnZWEEZDFRqZ5jQa5/ werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACBVzvfwDDFcK6ijMPfNLZJPy7gq52VhBGQxUameY0GufwAAAJA53OBlOdzg\nZQAAAAtzc2gtZWQyNTUxOQAAACBVzvfwDDFcK6ijMPfNLZJPy7gq52VhBGQxUameY0Gufw\nAAAEASHjc0wsfaUBKeyRYER9F+57Wbs0vHbiHaLcdwzQPBpVXO9/AMMVwrqKMw980tkk/L\nuCrnZWEEZDFRqZ5jQa5/AAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('62476ee0-c9ea-46a9-8541-440ac13e00ff', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Experiments server', '', false, true, false, 103, '{"alias":"kgY7us","address":"127.0.0.7","port":2207,"password":{"login":"ubuntu7","password":"ubuntu7"}}'),
('7bd860e6-df0d-45f6-a6b7-41251e73a07d', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'MySQL', '', true, false, false, 103, '{"alias":"kgY7us","address":"127.0.0.6","port":2922,"key":{"login":"test","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKbsyPIUlwnUsD0WMqO3lIYru+kDOQN3BOlXD/OmzQTT werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACCm7MjyFJcJ1LA9FjKjt5SGK7vpAzkDdwTpVw/zps0E0wAAAJAtOSz5LTks\n+QAAAAtzc2gtZWQyNTUxOQAAACCm7MjyFJcJ1LA9FjKjt5SGK7vpAzkDdwTpVw/zps0E0w\nAAAEAgnvZTmmFQEUYzUSo4qqFSrNtmUlFpPWblzDZ5vPTvCabsyPIUlwnUsD0WMqO3lIYr\nu+kDOQN3BOlXD/OmzQTTAAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('7cf11f1d-e31b-4d78-9339-90958bd82244', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Storage server backup', '', true, false, false, 103, '{"alias":"JiepgT","address":"127.0.0.4","port":2207,"password":{"login":"test","password":"test"}}'),
('893f2fd6-73e3-4f44-9de7-049a975ac181', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Storage server #1', '', true, false, false, 103, '{"alias":"Zba5zm","address":"9df8:5b5d:411b:b2f5:3996:25a7:b895:5ce7","port":2208,"key":{"login":"storage9","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGS3sGctNnxhyBT2DQViha7nh4zL6R9CHhJUR5FMaU60 noname werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACBkt7BnLTZ8YcgU9g0FYoWu54eMy+kfQh4SVEeRTGlOtAAAAJBscJhcbHCY\nXAAAAAtzc2gtZWQyNTUxOQAAACBkt7BnLTZ8YcgU9g0FYoWu54eMy+kfQh4SVEeRTGlOtA\nAAAEC8PozZ+97GM/4VfhAXqp5KSBF36OvPETANm0c2IV2QE2S3sGctNnxhyBT2DQViha7n\nh4zL6R9CHhJUR5FMaU60AAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('9a71be05-3907-4e9a-9c61-8caf0e7b9069', 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2', 'PostgreSQL', '', true, false, false, 103, '{"alias":"01SZPa","address":"127.0.0.5","port":22,"key":{"login":"user","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMAfIp3BwRVRDEBzrzB+fI8tCHbrQd9qtFHLMY/2EE0h werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACDAHyKdwcEVUQxAc68wfnyPLQh260HfarRRyzGP9hBNIQAAAJC85WtKvOVr\nSgAAAAtzc2gtZWQyNTUxOQAAACDAHyKdwcEVUQxAc68wfnyPLQh260HfarRRyzGP9hBNIQ\nAAAEDukDI8N1+8dvVPiUWhW+eLJlcPseqkK0nxV2d7Djz/qsAfIp3BwRVRDEBzrzB+fI8t\nCHbrQd9qtFHLMY/2EE0hAAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('b635a7e0-3980-4776-843a-8823600cd2bc', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Storage server #2', '', false, false, true, 103, '{"alias":"FDhP9n","address":"127.0.0.9","port":2209,"key":{"login":"ubuntu9","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIP2tgxmtQynabj2Vz1ghThhRJYNxzeZl3OJTkGHm7lRF werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACD9rYMZrUMp2m49lc9YIU4YUSWDcc3mZdziU5Bh5u5URQAAAJCvm9pJr5va\nSQAAAAtzc2gtZWQyNTUxOQAAACD9rYMZrUMp2m49lc9YIU4YUSWDcc3mZdziU5Bh5u5URQ\nAAAEBGGq7oTtt35XmdwjM3i0j3t0Bu/48/Ucbm8DD+zNw46P2tgxmtQynabj2Vz1ghThhR\nJYNxzeZl3OJTkGHm7lRFAAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('c504c295-63c9-406b-93e1-000c5e64977e', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Develop server #3', '', true, false, false, 103, '{"alias":"IUkc7f","address":"127.0.0.3","port":22,"key":{"login":"user","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB0JO0wPjURoFQUZOSI+ZH4l1gtd98bcFqbZCmSlHeBd werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACAdCTtMD41EaBUFGTkiPmR+JdYLXffG3Bam2QpkpR3gXQAAAJDg6P3R4Oj9\n0QAAAAtzc2gtZWQyNTUxOQAAACAdCTtMD41EaBUFGTkiPmR+JdYLXffG3Bam2QpkpR3gXQ\nAAAEDURZnoX4UwWLcZY0xOpACRHlM4dJFwyKCQd/yfB/NxqB0JO0wPjURoFQUZOSI+ZH4l\n1gtd98bcFqbZCmSlHeBdAAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('c6e37277-0513-4f9c-8e71-2464ebd1a016', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Develop server #1', '', true, false, false, 103, '{"alias":"byU3NV","address":"127.0.0.10","port":2210,"password":{"login":"ubuntu10","password":"ubuntu10"}}'),
('c95bf4ae-811e-45f1-acf1-e71131ad7ced', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Message server', '', true, false, false, 103, '{"alias":"EP6TQI","address":"127.0.0.1","port":22,"key":{"login":"root","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGJGHEjtB5J6OULh6CGlEotTQdDlzO3cPGDcpgRQJsLK werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACBiRhxI7QeSejlC4eghpRKLU0HQ5czt3Dxg3KYEUCbCygAAAJADX6oTA1+q\nEwAAAAtzc2gtZWQyNTUxOQAAACBiRhxI7QeSejlC4eghpRKLU0HQ5czt3Dxg3KYEUCbCyg\nAAAEDXdJGPGgZVKcl4cak9YXPUMJvMslqjeBhbJ7ysakcM4mJGHEjtB5J6OULh6CGlEotT\nQdDlzO3cPGDcpgRQJsLKAAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('d7797dff-4fa6-4a16-bc5b-68e5f7deb6f8', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Backups', '', false, false, true, 103, '{"alias":"H6WhcL","address":"127.0.0.4","port":2204,"password":{"login":"ubuntu4","password":"ubuntu4"}}'),
('ddd084a5-7d91-4796-a133-feab4e653721', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'CI/CD', '', true, true, true, 103, '{"alias":"x8GlLZ","address":"4f06:bbfc:a98e:7a58:b6de:93f8:4bcd:bcda","port":2205,"key":{"login":"user3","key":{"public": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC7L+QVM2RILf1C8PlTZ9Ab2aqW6/LrIBWiK4ybiKLzG werbot.com", "private": "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACAuy/kFTNkSC39QvD5U2fQG9mqluvy6yAVoiuMm4ii8xgAAAJB69UHfevVB\n3wAAAAtzc2gtZWQyNTUxOQAAACAuy/kFTNkSC39QvD5U2fQG9mqluvy6yAVoiuMm4ii8xg\nAAAEA1u0O/iIvR8F5ax4tkjtfmHqwD6jvyIQF4cxFx7j58PC7L+QVM2RILf1C8PlTZ9Ab2\naqW6/LrIBWiK4ybiKLzGAAAABm5vbmFtZQECAwQFBgc=\n-----END OPENSSH PRIVATE KEY-----", "passphrase": ""}}}'),
('df2046ee-5932-437f-b825-e55399666e45', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Mail server', '', true, true, true, 103, '{"alias":"43SokQ","address":"127.0.0.3","port":2203,"password":{"login":"ubuntu3","password":"ubuntu3"}}'),
-- telnet
('1f503895-7d76-49f6-ab53-d60a650bcc1d', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Telnet connect #1', '', true, false, false, 104, '{"alias":"V0zZuo","address":"193.168.7.30","port":23,"access":{"login":"ubuntu1","password":"ubuntu1"}}'),
('ddd27697-f583-4068-ba70-ed4b5c257b81', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Telnet connect #2', '', true, false, false, 104, '{"alias":"7WDwb0","address":"193.168.7.31","port":23,"access":{"login":"ubuntu2","password":"ubuntu2"}}'),
('d5fc56ea-96ee-4af3-acbf-a126cc0aba69', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Telnet connect #3', '', true, false, false, 104, '{"alias":"aY3DW5","address":"193.168.7.32","port":23,"access":{"login":"ubuntu3","password":"ubuntu3"}}'),
('368fe614-781b-494a-aba8-8358ec0833b0', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Telnet connect #1', '', true, false, false, 104, '{"alias":"KyS9Yw","address":"193.168.7.33","port":23,"access":{"login":"ubuntu4","password":"ubuntu4"}}'),
('cecbb30e-eaf8-4c26-afeb-2855481e854b', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Telnet connect #4', '', true, false, false, 104, '{"alias":"ncwfX4","address":"193.168.7.34","port":23,"access":{"login":"ubuntu5","password":"ubuntu5"}}'),
('666e633a-2bbb-4d2f-8e7d-65bb954041e3', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Telnet connect #5', '', true, false, false, 104, '{"alias":"83kLUC","address":"193.168.7.35","port":23,"access":{"login":"ubuntu6","password":"ubuntu6"}}'),
--
-- databases:
-- mysql
('1ff08110-a958-40b6-8742-842436a67015', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Mysql server #1', '', true, false, false, 201, '{"alias":"gqit1D","address":"192.151.43.1","port":3306,"database_name":"database1","access":{"login":"base1","password":"password1"},"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('bacfa933-d701-4a73-8e8a-71777abebbf3', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Mysql server #2', '', true, false, false, 201, '{"alias":"DIi3d1","address":"192.151.43.2","port":3306,"database_name":"database2","access":{"login":"base2","password":"password2"},"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('0918e4c3-7f61-4c4e-99ed-800c9af0d265', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Mysql server #3', '', true, false, false, 201, '{"alias":"m4M7Uu","address":"192.151.43.3","port":3306,"database_name":"database3","access":{"login":"base3","password":"password3"},"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('bdf147c5-2d8e-4af1-8672-60e5a89e9d23', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Mysql server #1', '', true, false, false, 201, '{"alias":"T37itI","address":"192.151.43.4","port":3306,"database_name":"database4","access":{"login":"base4","password":"password4"},"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('82688e24-d913-4c4b-85e7-d291a9707fbb', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Mysql server #2', '', true, false, false, 201, '{"alias":"7mMsS4","address":"192.151.43.5","port":3306,"database_name":"database5","access":{"login":"base5","password":"password5"},"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('fbbf6868-864d-476a-8cc3-7b87fd3e47b4', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Mysql server #3', '', true, false, false, 201, '{"alias":"jnJ53N","address":"192.151.43.6","port":3306,"database_name":"database6","access":{"login":"base6","password":"password6"},"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
-- postgres
('8d02dc4c-a1ce-499c-bce9-7d4491360538', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'PstgresQL server #1', '', true, false, false, 202, '{"alias":"gn5G2N","address":"192.163.22.1","port":5432,"database_name":"database1","access":{"login":"base1","password":"password1"},"mtls":{"server_ca":"","client_cert":"","client_key":""},"server_name_mtls":""}'),
('6c7532e8-1106-4c89-8ad7-9183dc71aece', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'PstgresQL server #2', '', true, false, false, 202, '{"alias":"6B0rbR","address":"192.163.22.2","port":5432,"database_name":"database2","access":{"login":"base2","password":"password2"},"mtls":{"server_ca":"","client_cert":"","client_key":""},"server_name_mtls":""}'),
('ea5506bc-4280-4a2b-a801-d2fae598737a', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'PstgresQL server #3', '', true, false, false, 202, '{"alias":"DEed11","address":"192.163.22.3","port":5432,"database_name":"database3","access":{"login":"base3","password":"password3"},"mtls":{"server_ca":"","client_cert":"","client_key":""},"server_name_mtls":""}'),
('bc80708b-8db2-419c-913a-cea715edd988', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'PstgresQL server #1', '', true, false, false, 202, '{"alias":"CDdc01","address":"192.163.22.4","port":5432,"database_name":"database4","access":{"login":"base4","password":"password4"},"mtls":{"server_ca":"","client_cert":"","client_key":""},"server_name_mtls":""}'),
('ad21217a-9802-4563-b505-9c5c8a0461c5', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'PstgresQL server #2', '', true, false, false, 202, '{"alias":"iC0I3c","address":"192.163.22.5","port":5432,"database_name":"database5","access":{"login":"base5","password":"password5"},"mtls":{"server_ca":"","client_cert":"","client_key":""},"server_name_mtls":""}'),
('be8bfc45-1031-4675-b913-a2cbde027abf', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'PstgresQL server #3', '', true, false, false, 202, '{"alias":"DmM14d","address":"192.163.22.6","port":5432,"database_name":"database6","access":{"login":"base6","password":"password6"},"mtls":{"server_ca":"","client_cert":"","client_key":""},"server_name_mtls":""}'),
-- redis
('189cbe21-e534-4536-8a41-62a76faa060b', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Redis server #1', '', true, false, false, 203, '{"alias":"A9ya0Y","address":"192.22.138.1","port":6379,"access":{"login":"base1","password":"password1"},"tls_required":0}'),
('a729e450-c150-45dd-90c9-6a0a54fe36cb', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Redis server #2', '', true, false, false, 203, '{"alias":"jJ33jJ","address":"192.22.138.2","port":6379,"access":{"login":"base2","password":"password2"},"tls_required":1}'),
('6660e8e8-fe5c-4ccc-a8d7-aa5190b1e5b4', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Redis server #3', '', true, false, false, 203, '{"alias":"E9z1Ze","address":"192.22.138.3","port":6379,"access":{"login":"base3","password":"password3"},"tls_required":0}'),
('f8da6f8b-565c-48ce-aaa8-2bb859d8bd6b', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Redis server #1', '', true, false, false, 203, '{"alias":"jJ3h2H","address":"192.22.138.4","port":6379,"access":{"login":"base4","password":"password4"},"tls_required":1}'),
('a301dbcb-3758-4611-a09b-3958103d53d1', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Redis server #2', '', true, false, false, 203, '{"alias":"4lL9zZ","address":"192.22.138.5","port":6379,"access":{"login":"base5","password":"password5"},"tls_required":0}'),
('aa60e95c-4da9-4987-a364-4997e4ff2df6', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Redis server #3', '', true, false, false, 203, '{"alias":"kK3Ff2","address":"192.22.138.6","port":6379,"access":{"login":"base6","password":"password6"},"tls_required":1}'),
-- mongodb
('3f186ae4-3a0c-49f7-a71e-942204da6680', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'MongoDB server #1', '', true, false, false, 204, '{"alias":"BH02hb","address":"192.91.203.1","port":27017,"database_name":"database1","access":{"login":"base1","password":"password1"},"tls_required":1,"replica_connect":0}'),
('c13da370-1cb0-45ed-8b4f-872aedd4e0ce', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'MongoDB server #2', '', true, false, false, 204, '{"alias":"S1e6sE","address":"192.91.203.2","port":27017,"database_name":"database2","access":{"login":"base2","password":"password2"},"tls_required":0,"replica_connect":1}'),
('dcabe398-e14e-4c2f-b2ea-7d2b1a34b4c8', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'MongoDB server #3', '', true, false, false, 204, '{"alias":"6RR6rr","address":"192.91.203.3","port":27017,"database_name":"database3","access":{"login":"base3","password":"password3"},"tls_required":1,"replica_connect":0}'),
('bcd8bc9d-b7d4-4a0a-bd98-aad671ad8299', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'MongoDB server #1', '', true, false, false, 204, '{"alias":"6RSrs6","address":"192.91.203.4","port":27017,"database_name":"database4","access":{"login":"base4","password":"password4"},"tls_required":0,"replica_connect":1}'),
('29bd11f6-5124-4e55-a16c-3b08963bb1b7', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'MongoDB server #2', '', true, false, false, 204, '{"alias":"gqG62Q","address":"192.91.203.5","port":27017,"database_name":"database5","access":{"login":"base5","password":"password5"},"tls_required":1,"replica_connect":0}'),
('9664604d-c1d8-46ac-aa05-1973d5e1cbfe', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'MongoDB server #3', '', true, false, false, 204, '{"alias":"5nN8wW","address":"192.91.203.6","port":27017,"database_name":"database6","access":{"login":"base6","password":"password6"},"tls_required":0,"replica_connect":1}'),
-- elastic
('52c5d504-960d-4cdf-9fe5-977b6476b1f9', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Elastic server #1', '', true, false, false, 205, '{"alias":"Q26Gqg","address":"192.73.31.1","port":9300,"access":{"login":"base1","password":"password1"},"tls_required":1}'),
('a7672124-97ed-4952-8186-aa9fdb3eeb10', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Elastic server #2', '', true, false, false, 205, '{"alias":"hr62RH","address":"192.73.31.2","port":9300,"access":{"login":"base2","password":"password2"},"tls_required":0}'),
('c6480b93-d900-4983-8c50-0059bea0e792', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Elastic server #3', '', true, false, false, 205, '{"alias":"mM4i3I","address":"192.73.31.3","port":9300,"access":{"login":"base3","password":"password3"},"tls_required":1}'),
('2acb611c-4ab9-4540-954a-ddcfd81ee308', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Elastic server #1', '', true, false, false, 205, '{"alias":"7TrR6t","address":"192.73.31.4","port":9300,"access":{"login":"base4","password":"password4"},"tls_required":0}'),
('1c0455b7-3475-47ff-84c2-7f72d9188606', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Elastic server #2', '', true, false, false, 205, '{"alias":"u6U7qQ","address":"192.73.31.5","port":9300,"access":{"login":"base5","password":"password5"},"tls_required":1}'),
('0a7d81e5-da9a-45d6-b849-0e762e03cc74', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Elastic server #3', '', true, false, false, 205, '{"alias":"A90Yya","address":"192.73.31.6","port":9300,"access":{"login":"base6","password":"password6"},"tls_required":0}'),
-- dynamodb
('865d8b15-488b-472f-a327-9284b28c6aea', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'DynamoDB server #1', '', true, false, false, 206, '{"alias":"pOo55P","region":"us-east-2","api":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('e65c6348-33a7-47e9-834d-20faa97b3d5a', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'DynamoDB server #2', '', true, false, false, 206, '{"alias":"jm43MJ","region":"us-east-1","api":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('64acbabc-58a0-4029-a73c-db36381d2274', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'DynamoDB server #3', '', true, false, false, 206, '{"alias":"Q16eqE","region":"us-west-2","api":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('678e37c9-9c5e-420b-bd40-f88d3d9b7e9d', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'DynamoDB server #1', '', true, false, false, 206, '{"alias":"Aa2G0g","region":"us-west-1","api":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('657b7535-2837-46d4-b178-0a8a904f0f59', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'DynamoDB server #2', '', true, false, false, 206, '{"alias":"If2Fi3","region":"ca-west-1","api":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('2420fbed-2547-4007-8f42-108238478630', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'DynamoDB server #3', '', true, false, false, 206, '{"alias":"D8Ww1d","region":"eu-central-1","api":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
-- cassandra
('30dd53dc-8d7d-4bc6-ac5b-75629457786b', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Cassandra server #1', '', true, false, false, 207, '{"alias":"Cci30I","address":"192.41.229.1","port":7001,"access":{"login":"base1","password":"password1"},"tls_required":0}'),
('fd4689a2-2d89-46d6-909c-427586f1bb27', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Cassandra server #2', '', true, false, false, 207, '{"alias":"B0Rbr6","address":"192.41.229.2","port":7001,"access":{"login":"base2","password":"password2"},"tls_required":1}'),
('1c56b6b7-e21c-4c42-8e7a-1b74ae645145', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Cassandra server #3', '', true, false, false, 207, '{"alias":"lQ4qL6","address":"192.41.229.3","port":7001,"access":{"login":"base3","password":"password3"},"tls_required":0}'),
('284d9dfb-48e8-493a-a1bc-d7eff8cf1aad', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Cassandra server #1', '', true, false, false, 207, '{"alias":"AGg2a0","address":"192.41.229.4","port":7001,"access":{"login":"base4","password":"password4"},"tls_required":1}'),
('83b8851f-2947-403a-8ea9-a4c3e4fae733', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Cassandra server #2', '', true, false, false, 207, '{"alias":"z9N5nZ","address":"192.41.229.5","port":7001,"access":{"login":"base5","password":"password5"},"tls_required":0}'),
('5713c93c-6e30-40a1-9b91-1a683d15b394', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Cassandra server #3', '', true, false, false, 207, '{"alias":"3zk9KZ","address":"192.41.229.6","port":7001,"access":{"login":"base6","password":"password6"},"tls_required":1}'),
-- sqlserver
('c66bf34f-dd90-4900-91d8-e1e7b65d3757', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Microsoft sqlserver server #1', '', true, false, false, 208, '{"alias":"D8Ww1d","address":"192.9.9.1","port":1433,"access":{"login":"base1","password":"password1"},"schema":"schema1","default_database":0}'),
('34b1e4a2-bf5f-4d13-a390-bc9e0337537d', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Microsoft sqlserver server #2', '', true, false, false, 208, '{"alias":"B0U7ub","address":"192.9.9.2","port":1433,"access":{"login":"base2","password":"password2"},"schema":"schema2","default_database":1}'),
('94a305d9-e7cd-43cf-b68c-faac2a73340b', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Microsoft sqlserver server #3', '', true, false, false, 208, '{"alias":"EGe2g1","address":"192.9.9.3","port":1433,"access":{"login":"base3","password":"password3"},"schema":"schema3","default_database":0}'),
('7ce3223d-813d-4acb-a74f-1f4e6cf519f6', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Microsoft sqlserver server #1', '', true, false, false, 208, '{"alias":"Mm4Ff2","address":"192.9.9.4","port":1433,"access":{"login":"base4","password":"password4"},"schema":"schema4","default_database":1}'),
('9963e235-66b6-4460-8105-90e4753521fa', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Microsoft sqlserver server #2', '', true, false, false, 208, '{"alias":"GhH2g2","address":"192.9.9.5","port":1433,"access":{"login":"base5","password":"password5"},"schema":"schema5","default_database":0}'),
('669d6a33-06f2-4ce8-ab7d-b765a2dcd6cf', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Microsoft sqlserver server #3', '', true, false, false, 208, '{"alias":"Ak40Ka","address":"192.9.9.6","port":1433,"access":{"login":"base6","password":"password6"},"schema":"schema6","default_database":1}'),
-- snowflake
('98ff725b-e40b-4a35-9adb-a049f5648bf4', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Snowflake server #1', '', true, false, false, 209, '{"alias":"A0q6aQ","region":"us-west-2","access":{"login":"base1","password":"password1"},"schema":"schema1"}'),
('fc90591c-a8a7-4031-b7c2-2b728ee275f5', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Snowflake server #2', '', true, false, false, 209, '{"alias":"bBO05o","region":"us-east-2","access":{"login":"base2","password":"password2"},"schema":"schema2"}'),
('b6b15566-d29a-4446-94f4-576c4a8a556d', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Snowflake server #3', '', true, false, false, 209, '{"alias":"S2s6gG","region":"us-central1","access":{"login":"base3","password":"password3"},"schema":"schema3"}'),
('4c40f72c-b8e3-418d-8d25-a4fcce967b49', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Snowflake server #1', '', true, false, false, 209, '{"alias":"2yYGg9","region":"us-east4","access":{"login":"base4","password":"password4"},"schema":"schema4"}'),
('79d71484-8fa9-4c93-90d8-23ec52cc06f0', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Snowflake server #2', '', true, false, false, 209, '{"alias":"3x9iXI","region":"southcentralus","access":{"login":"base5","password":"password5"},"schema":"schema5"}'),
('c6bf4bb6-9fcb-409e-ae24-dbd264e74b93', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Snowflake server #3', '', true, false, false, 209, '{"alias":"78VtvT","region":"canadacentral","access":{"login":"base6","password":"password6"},"schema":"schema6"}'),
--
-- desktops:
-- rdp
('d15380de-5b87-459f-b7d7-5b883f25af04', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'RDP server #1', '', true, false, false, 301, '{"alias":"7TR6rt","address":"192.38.195.1","port":3389,"access":{"login":"login1","password":"password1"}}'),
('ea2c5746-92aa-4334-b84f-4f36304d8a2d', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'RDP server #2', '', true, false, false, 301, '{"alias":"hq6QH3","address":"192.38.195.2","port":3389,"access":{"login":"login2","password":"password2"}}'),
('2ec70df3-39ca-4f5d-a2be-6052c730c755', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'RDP server #3', '', true, false, false, 301, '{"alias":"W8gGw2","address":"192.38.195.3","port":3389,"access":{"login":"login3","password":"password3"}}'),
('53d95bc2-1629-4bcf-88ee-b1b386ad63f4', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'RDP server #1', '', true, false, false, 301, '{"alias":"7ltTL4","address":"192.38.195.4","port":3389,"access":{"login":"login4","password":"password4"}}'),
('588835f1-79cd-4de0-9e3e-43b93442df47', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'RDP server #4', '', true, false, false, 301, '{"alias":"CBc10b","address":"192.38.195.5","port":3389,"access":{"login":"login5","password":"password5"}}'),
('2e0a41f5-f27c-46fe-bb58-303e928d1f1b', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'RDP server #5', '', true, false, false, 301, '{"alias":"s6PpS7","address":"192.38.195.6","port":3389,"access":{"login":"login6","password":"password6"}}'),
-- vnc
('51fe1bff-8bd8-4664-998b-d91082ac1d67', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'VNC server #1', '', true, false, false, 302, '{"alias":"jJ3fF2","address":"192.31.135.1","port":5900,"access":{"login":"login1","password":"password1"}}'),
('3936c707-5db7-44bf-8174-cfabf4a5267e', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'VNC server #2', '', true, false, false, 302, '{"alias":"Dd3I1i","address":"192.31.135.1","port":5900,"access":{"login":"login2","password":"password2"}}'),
('5ff2d92c-dc5b-4e90-8ded-984e0ba52bfb', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'VNC server #3', '', true, false, false, 302, '{"alias":"8vHV2h","address":"192.31.135.1","port":5900,"access":{"login":"login3","password":"password3"}}'),
('a0df5dcf-13ad-470d-bb84-807726e6bb30', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'VNC server #1', '', true, false, false, 302, '{"alias":"ln54NL","address":"192.31.135.1","port":5900,"access":{"login":"login4","password":"password4"}}'),
('5bfd45fa-896a-4009-af08-089cd82879ed', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'VNC server #4', '', true, false, false, 302, '{"alias":"8v3ViI","address":"192.31.135.1","port":5900,"access":{"login":"login5","password":"password5"}}'),
('3729075c-be48-43fe-8f53-be9e68df12eb', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'VNC server #5', '', true, false, false, 302, '{"alias":"Llh4H2","address":"192.31.135.1","port":5900,"access":{"login":"login6","password":"password6"}}'),
--
-- containers:
-- docker
('a4687d7d-c395-4d8e-9300-6af7fa6eff73', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Docker cluster #1', '', true, false, false, 401, '{"alias":"79SxsX","address":"192.66.132.1","port":2375,"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('cd7b7e4e-1a78-424a-9d56-7b2298879030', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Docker cluster #2', '', true, false, false, 401, '{"alias":"79TxXt","address":"192.66.132.2","port":2375,"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('785d4fe6-0b57-4825-bb09-1d84f2c13aab', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Docker cluster #3', '', true, false, false, 401, '{"alias":"C1P6cp","address":"192.66.132.3","port":2375,"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('43019efd-201b-4c15-a735-7fba12acf3c2', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Docker cluster #1', '', true, false, false, 401, '{"alias":"m8M4Xx","address":"192.66.132.4","port":2375,"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('79ce188c-6d49-4880-b1ca-72c22f1a83ec', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Docker cluster #4', '', true, false, false, 401, '{"alias":"3w8jWJ","address":"192.66.132.5","port":2375,"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
('e3b12d14-4b09-422a-a4be-5a202bedba8e', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Docker cluster #5', '', true, false, false, 401, '{"alias":"69yQqY","address":"192.66.132.6","port":2375,"mtls":{"server_ca":"","client_cert":"","client_key":""}}'),
-- k8s
('8255b16c-2b63-4b32-af49-9b420f22603a', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'K8S cluster #1', '', true, false, false, 402, '{"alias":"jJI33i","address":"192.241.6.1","port":9000,"mtls":{"server_ca":"","client_cert":"","client_key":""},"healthcheck_namespace":""}'),
('c1eb16c5-d0f8-4ed7-8e15-2a48877d5e6d', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'K8S cluster #2', '', true, false, false, 402, '{"alias":"k3Tt7K","address":"192.241.6.2","port":9000,"mtls":{"server_ca":"","client_cert":"","client_key":""},"healthcheck_namespace":""}'),
('74a17cfb-e44b-467d-88d6-9db7dcbc9596', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'K8S cluster #3', '', true, false, false, 402, '{"alias":"5ow8OW","address":"192.241.6.3","port":9000,"mtls":{"server_ca":"","client_cert":"","client_key":""},"healthcheck_namespace":""}'),
('819e0cf7-b0e6-4cc3-86c9-d4a5a3aae807', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'K8S cluster #1', '', true, false, false, 402, '{"alias":"i36IpP","address":"192.241.6.4","port":9000,"mtls":{"server_ca":"","client_cert":"","client_key":""},"healthcheck_namespace":""}'),
('d001ec97-9840-45c5-9e0d-2e5ec9c4209e', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'K8S cluster #4', '', true, false, false, 402, '{"alias":"8DVdv1","address":"192.241.6.5","port":9000,"mtls":{"server_ca":"","client_cert":"","client_key":""},"healthcheck_namespace":""}'),
('25578fde-f90b-4175-922a-9d624a8ce2da', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'K8S cluster #5', '', true, false, false, 402, '{"alias":"m8WM4w","address":"192.241.6.6","port":9000,"mtls":{"server_ca":"","client_cert":"","client_key":""},"healthcheck_namespace":""}'),
--
-- clouds:
-- aws
('86dbad37-454c-42c1-8262-fe66d804e05f', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'AWS cluster #1', '', true, false, false, 501, '{"alias":"5pwW8P","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""},"healthcheck_region":""}'),
('adaeb949-e4fe-4c71-b753-5712070db769', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'AWS cluster #2', '', true, false, false, 501, '{"alias":"r5R6Oo","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""},"healthcheck_region":""}'),
('4dd38d5f-8546-4a6b-86df-866049affc2e', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'AWS cluster #3', '', true, false, false, 501, '{"alias":"D1q6dQ","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""},"healthcheck_region":""}'),
('a0648b42-20a4-461a-8d17-b86b5b837d82', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'AWS cluster #1', '', true, false, false, 501, '{"alias":"A0Qqa6","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""},"healthcheck_region":""}'),
('8611d838-e544-48e5-bc5c-0c8969b7459b', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'AWS cluster #4', '', true, false, false, 501, '{"alias":"kl44LK","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""},"healthcheck_region":""}'),
('397f3d8a-7df1-4f39-9f1c-176607bfa891', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'AWS cluster #5', '', true, false, false, 501, '{"alias":"8Www8W","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""},"healthcheck_region":""}'),
-- gcp
('def9af59-6c36-4dc1-bd70-ee32fab53858', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'GCP cluster #1', '', true, false, false, 502, '{"alias":"q56NnQ","service_account_keyfile":"","scopes":""}'),
('3b69f69b-a4b5-4ec6-a913-b749f4e97903', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'GCP cluster #2', '', true, false, false, 502, '{"alias":"s5SnN7","service_account_keyfile":"","scopes":""}'),
('45c82fb3-11c9-44e9-8f51-c1e6fbc4c936', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'GCP cluster #3', '', true, false, false, 502, '{"alias":"lm4M4L","service_account_keyfile":"","scopes":""}'),
('5e7f70f1-6a3d-4a75-b610-4d3ebb21bcee', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'GCP cluster #1', '', true, false, false, 502, '{"alias":"q6QRr6","service_account_keyfile":"","scopes":""}'),
('9bd293a9-b0b6-42aa-a08b-329b036f8d7e', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'GCP cluster #4', '', true, false, false, 502, '{"alias":"0CWc8w","service_account_keyfile":"","scopes":""}'),
('7149d847-0ba7-4e69-8d79-f36a39d811bc', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'GCP cluster #5', '', true, false, false, 502, '{"alias":"W82fwF","service_account_keyfile":"","scopes":""}'),
-- azure
('286d55bd-139c-4e35-9337-83333eb52156', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Azure cluster #1', '', true, false, false, 503, '{"alias":"z9QqZ6","app_id":"","tenant":"","password":"","certificate":""}'),
('ce7e00c0-e5d9-42ff-acc2-c3693ce977b5', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Azure cluster #2', '', true, false, false, 503, '{"alias":"DDd1d1","app_id":"","tenant":"","password":"","certificate":""}'),
('2612f6d9-26a2-41fb-9b84-b10d3f41e196', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Azure cluster #3', '', true, false, false, 503, '{"alias":"78uVUv","app_id":"","tenant":"","password":"","certificate":""}'),
('5d386062-9397-457d-aef9-20e89a3b32e4', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Azure cluster #1', '', true, false, false, 503, '{"alias":"78uVUv","app_id":"","tenant":"","password":"","certificate":""}'),
('5873b942-f748-4bc3-93bf-c101741c85e4', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Azure cluster #4', '', true, false, false, 503, '{"alias":"1DZz9d","app_id":"","tenant":"","password":"","certificate":""}'),
('a680eeb3-f97e-47f2-bcc0-2726301305b1', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Azure cluster #5', '', true, false, false, 503, '{"alias":"Mm1Ee4","app_id":"","tenant":"","password":"","certificate":""}'),
-- do
('eb82728f-77c9-4743-9c7b-b8762a4be21f', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Digital ocean cluster #1', '', true, false, false, 504, '{"alias":"A8a0vV","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('912907de-cd85-4b2e-9666-1255ee408acb', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Digital ocean cluster #2', '', true, false, false, 504, '{"alias":"oO5o5O","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('d999c4ab-e205-4f9b-8355-bca36e51bc4b', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Digital ocean cluster #3', '', true, false, false, 504, '{"alias":"78xSsX","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('55d17731-053f-4f83-bad9-d4295f91cede', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Digital ocean cluster #1', '', true, false, false, 504, '{"alias":"kCcK40","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('2bec51f3-8a02-45dc-b717-858bf2db7fd4', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Digital ocean cluster #4', '', true, false, false, 504, '{"alias":"Dd13jJ","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('4eb14d73-9393-4ad0-8945-330ddddb0775', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Digital ocean cluster #5', '', true, false, false, 504, '{"alias":"6SQ6sq","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
-- hetzner
('ed63cc90-70e5-4ad4-bd51-c3aaf866125c', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Hetzner cluster #1', '', true, false, false, 505, '{"alias":"T2Ggt7","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('68785f36-7b16-4e9c-99ee-2955a98a56fe', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Hetzner cluster #2', '', true, false, false, 505, '{"alias":"7mS4sM","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('e7f3bfe6-41e8-49b9-a5b4-e32e0383e37a', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Hetzner cluster #3', '', true, false, false, 505, '{"alias":"Ooi5I3","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('08be4342-15c3-4d0a-844c-d140b46e4224', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Hetzner cluster #1', '', true, false, false, 505, '{"alias":"Z9z2Ff","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('e93d3068-dad0-4b75-9547-f115813c59be', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Hetzner cluster #4', '', true, false, false, 505, '{"alias":"DmMd14","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
('abff9bd4-80c0-4699-b5b2-aa5861e5027c', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Hetzner cluster #5', '', true, false, false, 505, '{"alias":"l84LXx","access":{"access_key_id":"","secret_access_key":"","role_arn":"","role_external_id":""}}'),
--
-- applications:
-- site
('4df672b7-1097-41ed-98ad-e7ac317cd0b5', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Site #1', '', true, false, false, 601, '{"alias":"lLIi43"}'),
('a8a76ac9-82bd-443c-8024-e6014c08ff7f', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Site #2', '', true, false, false, 601, '{"alias":"8uU3Ii"}'),
('a3eebac9-7b14-429b-bbfb-6ddb6a1a6a31', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 'Site #3', '', true, false, false, 601, '{"alias":"lLi43I"}'),
('3e6f44c4-0115-47f4-980d-6b4989a0104c', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Site #1', '', true, false, false, 601, '{"alias":"Cc2Gg1"}'),
('9806d33a-e75b-4b0a-87d6-37f4b3aea4bb', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Site #4', '', true, false, false, 601, '{"alias":"Cc2Gg1"}'),
('b46440b1-02b0-4685-b160-ade82b2339cf', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 'Site #5', '', true, false, false, 601, '{"alias":"69XxRr"}');

UPDATE "scheme" SET "access_policy" = jsonb_set("access_policy", '{country}', '1') WHERE "id" = '156d8d65-cfe5-48a4-a636-198a5f509abf';
UPDATE "scheme" SET "access_policy" = jsonb_set("access_policy", '{country}', '1') WHERE "id" = '7cf11f1d-e31b-4d78-9339-90958bd82244';

UPDATE "scheme_activity" SET "data" = '{"mon":[0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1], "tue":[1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0], "wed":[1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1], "thu":[1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1], "fri":[1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1], "sat":[1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1], "sun":[1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1]}' WHERE "scheme_id" = '156d8d65-cfe5-48a4-a636-198a5f509abf';
--UPDATE "scheme_activity" SET "data" = '{"mon":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "tue":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "wed":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "thu":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "fri":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "sat":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "sun":[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]}' WHERE "scheme_id" = '156d8d65-cfe5-48a4-a636-198a5f509abf';
UPDATE "scheme_activity" SET "data" = '{"mon":[1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1], "tue":[1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1], "wed":[1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1], "thu":[1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1], "fri":[1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1], "sat":[1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0], "sun":[0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1]}' WHERE "scheme_id" = '7cf11f1d-e31b-4d78-9339-90958bd82244';

INSERT INTO "scheme_firewall_country" ("id", "scheme_id", "country_code") VALUES
('1945a6ce-17f7-41ec-a39b-5fde9c08803f', '156d8d65-cfe5-48a4-a636-198a5f509abf', 'CN'),
('40b8a5e9-b042-4827-a8a6-4d477e59c92d', '156d8d65-cfe5-48a4-a636-198a5f509abf', 'RU'),
('6b5928b6-8546-4274-8a82-508dabe24517', '156d8d65-cfe5-48a4-a636-198a5f509abf', 'US'),
('934e771f-c6a2-4af1-81aa-6ea15c34264e', '7cf11f1d-e31b-4d78-9339-90958bd82244', 'BY'),
('5e6977a4-0c77-4379-b76c-0744666a0ccd', '7cf11f1d-e31b-4d78-9339-90958bd82244', 'LT'),
('4840eafd-ffcd-4a9a-bf26-2263db96e03e', '7cf11f1d-e31b-4d78-9339-90958bd82244', 'PL'),
('3d38c340-85c6-41d2-a25d-151bd7665e73', '7cf11f1d-e31b-4d78-9339-90958bd82244', 'LV');

INSERT INTO "scheme_firewall_network" ("id", "scheme_id", "network") VALUES
('43c0ee09-5e4a-4f79-a060-aa58fed6d813', '156d8d65-cfe5-48a4-a636-198a5f509abf', '192.168.1.0/24'),
('93a1fafb-9ed8-4fc4-b943-bc2c34e6f97b', '156d8d65-cfe5-48a4-a636-198a5f509abf', '127.0.0.3/32'),
('f0290ddc-1b69-432e-8dae-73c994410f10', '7cf11f1d-e31b-4d78-9339-90958bd82244', '192.168.1.0/24'),
('095bd120-b36c-42e3-a3ed-7624b09d1227', '7cf11f1d-e31b-4d78-9339-90958bd82244', '192.168.33.1/32'),
('28bc0200-7b4b-4862-a3a9-7a00068f2be4', '7cf11f1d-e31b-4d78-9339-90958bd82244', '127.0.0.3/32');

INSERT INTO "scheme_member" ("id", "scheme_id", "project_member_id", "active", "online") VALUES
('8ca0ad93-4338-44cb-93dd-3f57272d0ffa', '156d8d65-cfe5-48a4-a636-198a5f509abf', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false), -- admin owner, user member
('c0a487f1-6687-4c74-944a-39a687342fed', 'd0fa7301-dffa-444f-aca7-0124a3e8a17c', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('d145ad19-3703-4fb7-a9a6-0b718ca2fcff', '6fdc0d40-5055-4483-9045-e925f338751e', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('d56a101c-016e-4adb-847c-9318509efcc0', '62476ee0-c9ea-46a9-8541-440ac13e00ff', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('77f00769-063e-4f42-be48-5831b4ac1ad3', 'ddd27697-f583-4068-ba70-ed4b5c257b81', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('2f101a86-7479-4da1-834c-4533b5c23b3b', '1ff08110-a958-40b6-8742-842436a67015', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('d6e4d7ec-964b-4d42-a314-1e6349f6e08a', '6c7532e8-1106-4c89-8ad7-9183dc71aece', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, true),
('1425d7a7-cabd-4b50-9f1e-143d175e4520', 'a729e450-c150-45dd-90c9-6a0a54fe36cb', '9d3f7efc-14a5-436d-a763-314441d6e0a5', false, false),
('749b32cc-5b87-427c-96a2-7d7464b2cfab', '3f186ae4-3a0c-49f7-a71e-942204da6680', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('987bb535-4257-445c-b463-d124435bc2a9', 'a7672124-97ed-4952-8186-aa9fdb3eeb10', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('b59e2044-a931-457a-bbc3-682391186747', '865d8b15-488b-472f-a327-9284b28c6aea', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('3186e9e0-7786-41fc-a89f-5c3e1080f84b', '30dd53dc-8d7d-4bc6-ac5b-75629457786b', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('c474bff7-cfb4-41f3-8b16-82c9acc06cbf', 'c66bf34f-dd90-4900-91d8-e1e7b65d3757', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('e1ac11be-f9e8-4a4a-8058-30fd2a0b064d', '94a305d9-e7cd-43cf-b68c-faac2a73340b', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, true),
('88304f53-c29b-41ac-bbc5-8c256fa7e977', '98ff725b-e40b-4a35-9adb-a049f5648bf4', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('e3843b7f-c971-4c92-8cb8-86a5ede1f24e', 'd15380de-5b87-459f-b7d7-5b883f25af04', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('ab159e48-f687-4f65-b76f-b317b03b8307', '51fe1bff-8bd8-4664-998b-d91082ac1d67', '9d3f7efc-14a5-436d-a763-314441d6e0a5', false, false),
('366a7d3c-705d-4cf5-b51f-1efac4459c01', 'a4687d7d-c395-4d8e-9300-6af7fa6eff73', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('f6d95567-02a8-44fa-9fcf-556c3009e553', 'cd7b7e4e-1a78-424a-9d56-7b2298879030', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('ae933c12-c6ac-4cc9-8c1b-a764668c1c1c', '8255b16c-2b63-4b32-af49-9b420f22603a', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('9ee8c042-02c7-444b-81ea-064aef194b82', '86dbad37-454c-42c1-8262-fe66d804e05f', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('827897ca-b029-461b-83f3-b66af1128020', '4dd38d5f-8546-4a6b-86df-866049affc2e', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('d0b92827-0342-44fb-a661-7b8615231442', '3b69f69b-a4b5-4ec6-a913-b749f4e97903', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('407051af-0e15-4819-8436-dd914a1fa76c', 'ce7e00c0-e5d9-42ff-acc2-c3693ce977b5', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('41b38b05-1634-4ee6-89b0-9d9ccc044f57', 'eb82728f-77c9-4743-9c7b-b8762a4be21f', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('bee29fc7-3140-4184-b09d-f0a00eeb0f43', 'ed63cc90-70e5-4ad4-bd51-c3aaf866125c', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, true),
('20a69e89-eab7-46fb-8bb1-41ae90a18792', '68785f36-7b16-4e9c-99ee-2955a98a56fe', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('2422993b-89a7-4074-9b8e-63a1008944da', '4df672b7-1097-41ed-98ad-e7ac317cd0b5', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('568a8820-65d0-48a1-afbe-6c31879e69ec', 'eb82728f-77c9-4743-9c7b-b8762a4be21f', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
('be46b221-6ad5-4d8b-b720-c13259780f0b', 'a3eebac9-7b14-429b-bbfb-6ddb6a1a6a31', '9d3f7efc-14a5-436d-a763-314441d6e0a5', true, false),
--
('57ea9d56-5382-4749-99bb-b71a38d448b0', '7cf11f1d-e31b-4d78-9339-90958bd82244', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),  -- user owner, admin member
('a0d3e90c-362e-409b-aa15-5e5bc52699f9', 'd3128cea-0359-4104-bed1-d9497b174f31', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('5bae1864-0bcc-4205-85d9-f8debf5686d1', '6a77b657-cf64-44d2-b399-a28fd7d7c603', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('e36c87c1-ada6-41f9-9e70-b61989534052', 'c504c295-63c9-406b-93e1-000c5e64977e', '4fc69519-b683-46f0-860c-3e7f12a17563', true, true),
('fdcecf37-0bee-4ba9-9f5a-e8c0b7d9966d', 'cecbb30e-eaf8-4c26-afeb-2855481e854b', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('6ddcd475-401b-4a46-8bd2-f7066b8bf050', 'fbbf6868-864d-476a-8cc3-7b87fd3e47b4', '4fc69519-b683-46f0-860c-3e7f12a17563', false, false),
('9a46a32c-7088-4831-b22c-10e9e2890de6', 'ad21217a-9802-4563-b505-9c5c8a0461c5', '4fc69519-b683-46f0-860c-3e7f12a17563', true, true),
('4c3daaf6-5963-4f95-ba0b-1ba82369d0d0', 'be8bfc45-1031-4675-b913-a2cbde027abf', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('0ce4baa3-72c6-4ef6-952f-21002117b469', 'a301dbcb-3758-4611-a09b-3958103d53d1', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('9d2286f0-c55e-4f60-b15d-4e10d3eda91c', '9664604d-c1d8-46ac-aa05-1973d5e1cbfe', '4fc69519-b683-46f0-860c-3e7f12a17563', false, false),
('7b991097-dd3f-4da0-bac1-fbab92c9043e', '2acb611c-4ab9-4540-954a-ddcfd81ee308', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('320a602c-b6b3-4e95-b479-72b4d120b1e5', '657b7535-2837-46d4-b178-0a8a904f0f59', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('45e4de32-7a65-43ed-acd5-38bdc07bb0fd', '588835f1-79cd-4de0-9e3e-43b93442df47', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('ba5c4a6a-e137-4701-aada-9f89ca116482', '2e0a41f5-f27c-46fe-bb58-303e928d1f1b', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('207cf712-941f-46c8-bb74-6aa4cb9c6995', 'e3b12d14-4b09-422a-a4be-5a202bedba8e', '4fc69519-b683-46f0-860c-3e7f12a17563', true, true),
('b94c6d16-5c98-416d-b912-77d00b497e35', '79ce188c-6d49-4880-b1ca-72c22f1a83ec', '4fc69519-b683-46f0-860c-3e7f12a17563', true, false),
('dd4c2d8e-4da6-4474-86d7-b60b7e95964f', 'c504c295-63c9-406b-93e1-000c5e64977e', '49a10a09-0bb3-48af-99cb-181533692585', true, false), -- user owner, user1 member
('ba7eedb1-fd98-474a-b96f-4c0364f4cb47', 'c504c295-63c9-406b-93e1-000c5e64977e', '43ab80dd-ffe6-4881-aa8d-52b56ea715d2', true, true); -- user owner, user2 member

UPDATE "scheme_member" SET "online" = true WHERE "id" = '57ea9d56-5382-4749-99bb-b71a38d448b0';

INSERT INTO "event_profile" ("id", "user_id", "profile_id", "session_id", "user_agent", "ip", "event", "section", "data") VALUES
('59fab0fa-8f0a-4065-8863-0dae40166015', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:35.0) Gecko/20100101 Firefox/35.', '2001:0db8:85a3:0000:0000:8a2e:0370:7334', 9, 1, '{}'),
('7c1bd7f9-2ef4-44c8-9756-0e85156ca58f', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 AppleWebKit/999.0 (KHTML, like Gecko) Chrome/99.0 Safari/999.0', '192.168.1.1', 10, 1, '{}');

INSERT INTO "event_project" ("id", "project_id", "profile_id", "session_id", "user_agent", "ip", "event", "section", "data") VALUES
('163dee10-2a74-4436-9507-65a97a711ba8', '26060c68-5a06-4a57-b87a-be0f1e787157', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; KFJWI Build/IMM76D) AppleWebKit/537.36 (KHTML, like Gecko) Silk/3.68 like Chrome/39.0.2171.93 Safari/537.36', '192.168.0.1', 1, 1, '{}'),
('9758b5ee-367d-4a70-965b-14a129cca4d7', '26060c68-5a06-4a57-b87a-be0f1e787157', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.10; rv:39.0) Gecko/20100101 Firefox/39.0', '2001:0db8:85a3:0000:0000:8a2e:0370:7334', 2, 1, '{}');

INSERT INTO "event_scheme" ("id", "scheme_id", "profile_id", "session_id", "user_agent", "ip", "event", "section", "data") VALUES
('dea438b3-ca64-45ad-80a6-51275730f078', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; LCJB; rv:11.0) like Gecko', '192.168.1.1', 1, 1, '{}'),
('a2ef053e-4124-487b-9e90-b8f249d49807', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/44.0.2403.157 Safari/537.36', '2001:0db8:85a3:0000:0000:8a2e:0370:7334', 2, 1, '{}'),
('fafa6e1f-1b13-47de-bc6a-df2458c29ff8', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (iPad; CPU OS 8_0_2 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) Version/8.0 Mobile/12A405 Safari/600.1.4', '192.168.1.1', 3, 1, '{}'),
('ac79f9c0-7bce-4179-ba3c-ed9ea4ecb14f', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Windows NT 6.1; rv:38.0) Gecko/20100101 Firefox/38.0', '2001:0db8:0a0b:12f0:0000:0000:0000:0001', 4, 1, '{}'),
('39c7c278-cccb-4353-8df0-991a87df343f', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (X11; Linux x86_64; rv:31.0) Gecko/20100101 Firefox/31.0', '192.168.1.1', 5, 1, '{}'),
('e20a3310-20df-457d-8122-ecfab36fd8c5', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:39.0) Gecko/20100101 Firefox/39.0', '2001:db8:a0b:12f0::1', 6, 1, '{}'),
('5aedee8e-5e18-451a-9b71-c65c99697364', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; MDDCJS; rv:11.0) like Gecko', '192.168.1.2', 7, 1, '{}'),
('0b1df8d7-c0cd-4a48-bcfc-248b2abe0c93', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (iPhone; CPU iPhone OS 7_1_2 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D257 Safari/9537.53', '2001:db8:a0b:12f0::0:0:1', 8, 1, '{}'),
('d1ce2bb8-d3d4-4406-9951-01ffa63d3c7f', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (iPad; CPU OS 7_0_2 like Mac OS X) AppleWebKit/537.51.1 (KHTML, like Gecko) Version/7.0 Mobile/11A501 Safari/9537.53', '192.168.1.3', 1, 1, '{}'),
('c4ab0740-b38f-40f6-9de1-00d54412b491', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (Windows NT 10.0; WOW64; rv:39.0) Gecko/20100101 Firefox/39.0', '192.168.1.4', 2, 1, '{}'),
('30d3a040-57c7-4613-b500-23197ffa600e', '0c3a8869-6fc0-4666-bf60-15475473392a', '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686', '98E3DDFC-DAB0-4D4E-B48E-AB1717ACAE8B', 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/534.24 (KHTML, like Gecko) Chrome/33.0.0.0 Safari/534.24', '192.168.1.5', 1, 1, '{}');

INSERT INTO "agent_token" ("token", "project_id", "scheme_type", "expired") VALUES
('39c45364-4b94-45bb-88b4-1360245f8a59', '2bef1080-cd6e-49e5-8042-1224cf6a3da9', 103, CURRENT_TIMESTAMP + INTERVAL '1 month'), -- admin, ssh
('0a177fc3-ad38-40c6-b936-ded649ce5a57', 'd958ee44-a960-420e-9bbf-c7a35084c4aa', 103, CURRENT_TIMESTAMP + INTERVAL '1 month'); -- user, ssh

INSERT INTO "firewall_country" ("firewall_list_id", "country_code") VALUES
('afe7547c-f7af-41f0-bf80-dd72b807834f', 'RU'),
('afe7547c-f7af-41f0-bf80-dd72b807834f', 'BY');

INSERT INTO "firewall_network" ("firewall_list_id", "network") VALUES
 ('afe7547c-f7af-41f0-bf80-dd72b807834f', '178.239.2.11/32');

UPDATE "scheme_host_key" SET "host_key" = '\x746573740A' WHERE "scheme_id" = '156d8d65-cfe5-48a4-a636-198a5f509abf';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE "scheme_host_key" SET "host_key" = '' WHERE "scheme_id" = '156d8d65-cfe5-48a4-a636-198a5f509abf';
DELETE FROM "firewall_network";
DELETE FROM "firewall_country";
DELETE FROM "agent_token";
DELETE FROM "event_scheme";
DELETE FROM "event_project";
DELETE FROM "event_profile";
DELETE FROM "scheme_member";
DELETE FROM "scheme_firewall_network";
DELETE FROM "scheme_firewall_country";
DELETE FROM "scheme_activity";
DELETE FROM "scheme_host_key";
DELETE FROM "scheme";
DELETE FROM "project_member";
DELETE FROM "project_invite";
DELETE FROM "project_api";
DELETE FROM "project";
DELETE FROM "profile_public_key";
DELETE FROM "profile";
-- +goose StatementEnd
