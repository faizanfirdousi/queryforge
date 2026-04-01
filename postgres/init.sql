create table users (
    id bigserial primary key,
    name text not null,
    email text not null unique,
    created_at timestamptz default now()
);

create table transactions (
    id bigserial primary key,
    user_id bigint not null references users(id),
    amount decimal(10,2) not null,
    transaction_type text not null check (transaction_type IN ('credit', 'debit')),
    description text,
    created_at timestamptz default now()
);