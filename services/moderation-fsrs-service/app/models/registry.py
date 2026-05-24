"""Single source of truth for loaded models. Built once at startup (Rule 2)."""
from __future__ import annotations

from dataclasses import dataclass

from app.config import Settings
from app.models.image_moderator import ImageModerator
from app.models.text_moderator import TextModerator


@dataclass
class ModelRegistry:
    text: TextModerator
    image: ImageModerator


def build_registry(settings: Settings) -> ModelRegistry:
    return ModelRegistry(
        text=TextModerator(settings.text_model_dir, settings.text_threshold),
        image=ImageModerator(
            settings.image_model_dir,
            settings.image_threshold,
            fallback_id=settings.fallback_vit_id,
        ),
    )
