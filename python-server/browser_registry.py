"""Persistent browser registration storage."""

from __future__ import annotations

import json
from pathlib import Path
from threading import RLock
from typing import Dict, List, Optional


class BrowserRegistryError(ValueError):
    """Raised when a browser registration request is invalid."""


class BrowserRegistry:
    def __init__(self, path: Optional[Path] = None):
        self.path = path or Path(__file__).with_name("browser_registry.json")
        self._lock = RLock()
        self._browsers: Dict[str, str] = {}
        self._load()

    def _load(self) -> None:
        if not self.path.exists():
            return

        with self.path.open("r", encoding="utf-8") as file:
            data = json.load(file)

        browsers = data.get("browsers", []) if isinstance(data, dict) else []
        self._browsers = {
            str(item["connect_id"]): str(item["name"])
            for item in browsers
            if item.get("connect_id") and item.get("name")
        }

    def _save(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        data = {
            "browsers": [
                {"connect_id": connect_id, "name": name}
                for connect_id, name in sorted(self._browsers.items())
            ]
        }
        tmp_path = self.path.with_suffix(self.path.suffix + ".tmp")
        with tmp_path.open("w", encoding="utf-8") as file:
            json.dump(data, file, ensure_ascii=False, indent=2)
            file.write("\n")
        tmp_path.replace(self.path)

    def register(self, connect_id: str, name: str) -> Dict[str, str]:
        connect_id = connect_id.strip()
        name = name.strip()
        if not connect_id:
            raise BrowserRegistryError("connect_id is required")
        if not name:
            raise BrowserRegistryError("name is required")

        with self._lock:
            existing_name = self._browsers.get(connect_id)
            if existing_name is not None:
                if existing_name == name:
                    return {"connect_id": connect_id, "name": name}
                raise BrowserRegistryError("connect_id is already registered with a different name")

            for existing_id, existing in self._browsers.items():
                if existing == name and existing_id != connect_id:
                    raise BrowserRegistryError("name is already registered")

            self._browsers[connect_id] = name
            self._save()
            return {"connect_id": connect_id, "name": name}

    def validate(self, connect_id: str, name: str) -> bool:
        with self._lock:
            return self._browsers.get(connect_id) == name

    def get_name(self, connect_id: str) -> Optional[str]:
        with self._lock:
            return self._browsers.get(connect_id)

    def list_registered(self) -> List[Dict[str, str]]:
        with self._lock:
            return [
                {"connect_id": connect_id, "name": name}
                for connect_id, name in sorted(self._browsers.items())
            ]
