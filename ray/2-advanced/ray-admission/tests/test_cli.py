import unittest
from contextlib import redirect_stdout
from io import StringIO
from unittest.mock import patch

from ray_admission.cli import _implementations, _parser, main
from ray_admission.queue import _cli_resources


class CliTest(unittest.TestCase):
    def test_zero_quota_is_preserved(self):
        self.assertEqual(_cli_resources('{"GPU": 0}'), {"GPU": 0})

    def test_discovers_queue_submodule(self):
        self.assertIn("queue", _implementations())

    def test_defaults(self):
        args = _parser().parse_args(["queue", "start"])
        self.assertEqual(args.address, None)

    def test_identity_options_are_rejected(self):
        for option in ("--name", "--namespace"):
            with patch("sys.stderr", new=StringIO()):
                with self.assertRaises(SystemExit):
                    _parser().parse_args(["queue", option, "other", "start"])

    @patch("ray_admission.queue.ray")
    def test_list_does_not_connect_to_ray(self, ray):
        output = StringIO()
        with redirect_stdout(output):
            main(["list"])
        self.assertIn("queue\t", output.getvalue())
        ray.assert_not_called()
        ray.init.assert_not_called()

    def test_top_level_queue_commands_are_rejected(self):
        with redirect_stdout(StringIO()), patch("sys.stderr", new=StringIO()):
            with self.assertRaises(SystemExit):
                _parser().parse_args(["start"])

    @patch("ray_admission.queue.ray")
    @patch("ray_admission.queue.start")
    def test_start_uses_token_from_environment(self, start, ray):
        ray.is_initialized.return_value = False
        with patch.dict(
            "os.environ",
            {"RAY_ADMISSION_ADMIN_TOKEN": "secret", "RAY_NAMESPACE": "business"},
        ):
            main(["queue", "--address", "auto", "start"])
        ray.init.assert_called_once_with(
            address="auto", namespace="ray-submission-queue"
        )
        start.assert_called_once_with(
            admin_token="secret",
        )

    @patch("ray_admission.queue.ray")
    def test_quota_uses_environment_namespace(self, ray):
        ray.is_initialized.return_value = False
        with patch.dict(
            "os.environ",
            {"RAY_ADMISSION_ADMIN_TOKEN": "secret", "RAY_NAMESPACE": "business"},
        ):
            main(["queue", "quota", "set", "job-1", '{"CPU": 1}'])
        ray.get_actor.assert_called_once_with(
            "ray-submission-queue", namespace="ray-submission-queue"
        )
        ray.get_actor.return_value.set_job_quota.remote.assert_called_once_with(
            "job-1", {"CPU": 1}, quota_namespace="business", admin_token="secret"
        )
