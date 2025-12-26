import importlib
import os
import sys
import types
import unittest


class TestResolvePaths(unittest.TestCase):
    def setUp(self):
        self._original_env = os.environ.copy()

        os.environ["DISCORD_WEBHOOK_URL"] = "https://example.invalid/webhook"
        os.environ["RSS_BOT_CONFIG"] = "/nonexistent/config.env"

    def tearDown(self):
        os.environ.clear()
        os.environ.update(self._original_env)

    def _import_bot_service(self):
        for module_name in ("feedparser", "requests", "html2text"):
            if module_name not in sys.modules:
                sys.modules[module_name] = types.ModuleType(module_name)

        import bot_service

        return importlib.reload(bot_service)

    def test_prefers_rss_bot_vars(self):
        os.environ["RSS_BOT_DATA"] = "/app/data"
        os.environ["RSS_BOT_LOGS"] = "/app/logs"
        os.environ["RSS_BOT_PID"] = "/tmp/discord-rss-bot.pid"

        bot_service = self._import_bot_service()
        data_dir, log_dir, db_file, log_file, pid_file = bot_service.resolve_paths(os.environ, os_name="posix")

        self.assertEqual(data_dir, "/app/data")
        self.assertEqual(log_dir, "/app/logs")
        self.assertEqual(db_file, "/app/data/posted_hashes.db")
        self.assertEqual(log_file, "/app/logs/rss_bot.log")
        self.assertEqual(pid_file, "/tmp/discord-rss-bot.pid")

    def test_legacy_log_file_derives_log_dir(self):
        os.environ.pop("RSS_BOT_LOGS", None)
        os.environ["LOG_FILE"] = "/app/logs/custom.log"

        bot_service = self._import_bot_service()
        data_dir, log_dir, db_file, log_file, pid_file = bot_service.resolve_paths(os.environ, os_name="posix")

        self.assertEqual(log_dir, "/app/logs")
        self.assertEqual(log_file, "/app/logs/custom.log")

    def test_legacy_data_dir_used_for_default_db(self):
        os.environ.pop("RSS_BOT_DATA", None)
        os.environ["DATA_DIR"] = "/custom/data"

        bot_service = self._import_bot_service()
        data_dir, log_dir, db_file, log_file, pid_file = bot_service.resolve_paths(os.environ, os_name="posix")

        self.assertEqual(data_dir, "/custom/data")
        self.assertEqual(db_file, "/custom/data/posted_hashes.db")

    def test_db_file_override(self):
        os.environ["RSS_BOT_DATA"] = "/app/data"
        os.environ["DB_FILE"] = "/var/lib/discord-rss-bot/posted_hashes.db"

        bot_service = self._import_bot_service()
        data_dir, log_dir, db_file, log_file, pid_file = bot_service.resolve_paths(os.environ, os_name="posix")

        self.assertEqual(data_dir, "/app/data")
        self.assertEqual(db_file, "/var/lib/discord-rss-bot/posted_hashes.db")


if __name__ == "__main__":
    unittest.main()
