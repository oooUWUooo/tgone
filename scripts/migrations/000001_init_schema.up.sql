-- Create sources table
CREATE TABLE IF NOT EXISTS sources (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(255) NOT NULL UNIQUE,
    category VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create articles table
CREATE TABLE IF NOT EXISTS articles (
    id SERIAL PRIMARY KEY,
    guid VARCHAR(255) NOT NULL UNIQUE,
    title TEXT NOT NULL,
    link TEXT NOT NULL,
    summary TEXT,
    image_url TEXT,
    pub_date TIMESTAMP WITH TIME ZONE,
    source_id INTEGER REFERENCES sources(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial Habr InfoSec source
INSERT INTO sources (name, url, category)
VALUES ('Habr InfoSec', 'https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru', 'infosecurity')
ON CONFLICT (url) DO NOTHING;

-- Insert another source for "Large Project" feel
INSERT INTO sources (name, url, category)
VALUES ('Habr IT News', 'https://habr.com/ru/rss/hubs/all/', 'it')
ON CONFLICT (url) DO NOTHING;
