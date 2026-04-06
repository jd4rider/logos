#!/usr/bin/env python3
import os
import sys
from pathlib import Path

import numpy as np
from kokoro_onnx import Kokoro


def kokoro_dir() -> Path:
    override = os.environ.get("LOGOS_KOKORO_DIR", "").strip()
    if override:
        return Path(override)
    return Path.home() / ".local" / "share" / "logos" / "kokoro"


def language_for_voice(voice: str) -> str:
    voice = (voice or "").lower()
    if voice.startswith(("bf_", "bm_")):
        return "en-gb"
    return "en-us"


def main() -> int:
    root = kokoro_dir()
    model = root / "kokoro-v1.0.onnx"
    voices = root / "voices-v1.0.bin"

    if not model.exists():
        raise SystemExit(f"Missing Kokoro model: {model}")
    if not voices.exists():
        raise SystemExit(f"Missing Kokoro voices: {voices}")

    voice = sys.argv[1] if len(sys.argv) > 1 else "af_heart"
    speed = float(sys.argv[2]) if len(sys.argv) > 2 else 1.0
    text = sys.stdin.read().strip()
    if not text:
        return 0

    engine = Kokoro(str(model), str(voices))
    samples, _rate = engine.create(text, voice=voice, speed=speed, lang=language_for_voice(voice))
    pcm = (np.clip(samples, -1.0, 1.0) * 32767).astype(np.int16)
    sys.stdout.buffer.write(pcm.tobytes())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
