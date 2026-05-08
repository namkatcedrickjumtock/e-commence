
-- name: GetProduct :one
SELECT id, name, price_cents, stock
FROM products
WHERE id = $1;

-- name: ReserveStock :exec
UPDATE products
SET stock = stock - $2
WHERE id = $1 AND stock >= $2;

-- name: InsertCartItem :exec
INSERT INTO cart_items (product_id, quantity)
VALUES ($1, $2);

-- name: ListCartItems :many
SELECT product_id, quantity
FROM cart_items
ORDER BY product_id;

-- name: ClearCart :exec
DELETE FROM cart_items;

-- name: CreateOrder :exec
INSERT INTO orders (id, total_cents)
VALUES ($1, $2);

-- name: AddOrderItem :exec
INSERT INTO order_items (order_id, product_id, quantity)
VALUES ($1, $2, $3);
