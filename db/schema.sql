-- Minimal schema to support demo use-cases.
-- Engine: PostgreSQL (chosen for sqlc demo familiarity).

CREATE TABLE IF NOT EXISTS products (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  price_cents integer NOT NULL,
  stock       integer NOT NULL CHECK (stock >= 0)
);

CREATE TABLE IF NOT EXISTS cart_items (
  product_id  text PRIMARY KEY REFERENCES products(id),
  quantity    integer NOT NULL CHECK (quantity > 0)
);

CREATE TABLE IF NOT EXISTS orders (
  id          text PRIMARY KEY,
  total_cents integer NOT NULL
);

CREATE TABLE IF NOT EXISTS order_items (
  order_id    text NOT NULL REFERENCES orders(id),
  product_id  text NOT NULL REFERENCES products(id),
  quantity    integer NOT NULL CHECK (quantity > 0),
  PRIMARY KEY (order_id, product_id)
); 


