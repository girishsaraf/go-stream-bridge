-- Create the messages table used by the SQL Server sink.
IF OBJECT_ID(N'dbo.messages', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.messages (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        message NVARCHAR(MAX) NOT NULL,
        timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
END
