-- Destructive rollback: staff accounts and sessions are removed. Back up first.
DROP TABLE auth_session;
DROP TABLE worker;
DROP TABLE admin;
DROP TABLE staff_account;
DROP TABLE person;
