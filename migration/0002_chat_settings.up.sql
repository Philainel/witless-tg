DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'chat_status') THEN
		CREATE TYPE chat_status AS ENUM ('on', 'learning', 'messaging', 'off');
	END IF;
END
$$;

CREATE TABLE IF NOT EXISTS chat (
	chat BIGINT PRIMARY KEY,
	rate SMALLINT DEFAULT 90,
	working_mode chat_status DEFAULT 'on'
);

