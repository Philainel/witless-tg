CREATE TABLE IF NOT EXISTS token (
	id BIGSERIAL PRIMARY KEY,
	word TEXT NOT NULL UNIQUE
);

-- Ensure there's in-system tokens START and END
INSERT INTO token (id, word)
VALUES (1, E'\x1f')
ON CONFLICT (id) DO NOTHING;
INSERT INTO token (id, word)
VALUES (2, E'\x1c')
ON CONFLICT (id) DO NOTHING;


CREATE TABLE IF NOT EXISTS links (
	token BIGINT,
	chat BIGINT,
	next BIGINT,
	count INT,

	FOREIGN KEY (token) REFERENCES token(id),
	FOREIGN KEY (next) REFERENCES token(id),
	PRIMARY KEY (token, chat, next)
);

