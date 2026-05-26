import json
import tempfile
import unittest
from pathlib import Path

from browser_registry import BrowserRegistry, BrowserRegistryError


class BrowserRegistryTest(unittest.TestCase):
    def new_registry(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        return BrowserRegistry(Path(temp_dir.name) / "browser_registry.json")

    def test_register_browser(self):
        registry = self.new_registry()

        browser = registry.register("browser-1", "chrome-office")

        self.assertEqual(browser, {"connect_id": "browser-1", "name": "chrome-office"})
        self.assertTrue(registry.validate("browser-1", "chrome-office"))

    def test_register_is_idempotent_for_same_id_and_name(self):
        registry = self.new_registry()
        registry.register("browser-1", "chrome-office")

        browser = registry.register("browser-1", "chrome-office")

        self.assertEqual(browser, {"connect_id": "browser-1", "name": "chrome-office"})

    def test_rejects_duplicate_name(self):
        registry = self.new_registry()
        registry.register("browser-1", "chrome-office")

        with self.assertRaises(BrowserRegistryError):
            registry.register("browser-2", "chrome-office")

    def test_rejects_same_id_with_different_name(self):
        registry = self.new_registry()
        registry.register("browser-1", "chrome-office")

        with self.assertRaises(BrowserRegistryError):
            registry.register("browser-1", "chrome-home")

    def test_reload_from_json_file(self):
        registry = self.new_registry()
        registry.register("browser-1", "chrome-office")

        reloaded = BrowserRegistry(registry.path)

        self.assertEqual(reloaded.get_name("browser-1"), "chrome-office")
        self.assertEqual(
            json.loads(registry.path.read_text(encoding="utf-8")),
            {"browsers": [{"connect_id": "browser-1", "name": "chrome-office"}]},
        )


if __name__ == "__main__":
    unittest.main()
