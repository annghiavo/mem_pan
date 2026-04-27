CREATE TYPE card_language AS ENUM (
    'vi',
    'en',
    'es',
    'fr',
    'it',
    'de',
    'ru',
    'ja',
    'ja_romaji',
    'zh_hans',
    'zh_hant',
    'zh_pinyin',
    'ko'
);

ALTER TABLE notes
    ADD COLUMN lang_front card_language NOT NULL DEFAULT 'en',
    ADD COLUMN lang_back  card_language NOT NULL DEFAULT 'en';
