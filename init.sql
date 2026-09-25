CREATE TABLE employees (
    id SERIAL PRIMARY KEY,
    firstname TEXT NOT NULL,
    lastname TEXT NOT NULL,
    salary NUMERIC(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO employees (firstname, lastname, salary) VALUES
('Alice', 'Martin', 42000.00),
('Bob', 'Durand', 38000.00),
('Claire', 'Dupont', 45000.00);