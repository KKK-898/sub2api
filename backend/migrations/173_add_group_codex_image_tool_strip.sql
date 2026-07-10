ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS strip_codex_image_generation_tool BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.strip_codex_image_generation_tool IS
    'Strip Codex/Responses image generation tools before applying the OpenAI group image-generation gate';
