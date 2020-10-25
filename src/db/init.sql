CREATE TABLE if not exists auth_confirmation (
    email varchar(32) UNIQUE,
    hash varchar(128) UNIQUE,
    deadline TIMESTAMP WITH TIME ZONE
);

CREATE TABLE if not exists subscription (
    acc_verified bool,
    email varchar(32),
    price int,
    url varchar(128)
)
