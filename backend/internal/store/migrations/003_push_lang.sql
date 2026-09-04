-- Notification text is built on the server, so each phone records the language
-- it was subscribed in. Slovenian is the default, as everywhere else.
ALTER TABLE push_subscriptions ADD COLUMN lang TEXT NOT NULL DEFAULT 'sl';
