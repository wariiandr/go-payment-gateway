CREATE TABLE processed_commands (
    command_id VARCHAR(255) PRIMARY KEY,
    result VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
