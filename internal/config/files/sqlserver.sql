-- Create messages table
CREATE TABLE messages (
    id INT IDENTITY(1,1) PRIMARY KEY,
    message VARCHAR(255) NOT NULL,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
