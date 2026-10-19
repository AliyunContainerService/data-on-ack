#!/usr/bin/env python3
"""
Adjust the data source, output directory, model paths, and resource parameters as needed.
"""

from __future__ import annotations

import json
import os
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

# 1. Input: MP4 root accessible to every Ray pod. The code recursively finds *.mp4.
INPUT_DIR = "/data/oss/egodex/data/test"

# 2. Output root: an OSS RWX path writable by every Ray pod.
OUTPUT_ROOT = (
    "/data/output/vla-results/"
    "vla-demo"
)

# 3. Model root. See validate_models() for the required layout.
MODEL_ROOT = "/data/oss/models"

# 4. Data and export parameters. This demo processes the first 100 sorted MP4 files.
LIMIT = 100
TASK_TEXT = "manipulate object"
FRAME_NUM = 20
LEROBOT_FPS = 10

# 5. Ray Data resources. MoGe uses 8 actors; Combined scales from 8 to 14.
#    Each actor requests 0.25 GPU. GPU_BATCH_SIZE applies only to these stages.
CPU_PER_TASK = 1.0
GPU_BATCH_SIZE = 16
MOGE_GPUS = 0.25
COMBINED_GPUS = 0.25
MOGE_ACTOR_MIN = 8
MOGE_ACTOR_MAX = 8
COMBINED_ACTOR_MIN = 8
COMBINED_ACTOR_MAX = 14

# 6. Caption model. Inject OPENAI_API_KEY into the Ray head or pass it through
#    the RayJob runtime_env. Never hard-code it here. The mapper warns and falls
#    back to default text when credentials are unavailable.
CAPTION_MODEL = os.environ.get("VLA_CAPTION_MODEL", "qwen-vl-max")
CAPTION_API_ENDPOINT = os.environ.get("VLA_CAPTION_API_ENDPOINT", "")

VIDEO_KEY = "videos"
EXPECTED_DATA_JUICER_VERSION = "1.6.0"

import ray  # noqa: E402
from ray.data import ActorPoolStrategy  # noqa: E402

from data_juicer.ops.base_op import OPERATORS, Mapper  # noqa: E402
from data_juicer.ops.mapper import (  # noqa: E402
    ExportToLeRobotMapper,
    VideoCameraCalibrationMogeMapper,
    VideoCameraPoseMegaSaMMapper,
    VideoExtractFramesMapper,
    VideoHandActionComputeMapper,
    VideoHandReconstructionHaworMapper,
)
from data_juicer.utils.constant import Fields, MetaKeys  # noqa: E402

sys.path.insert(0, os.path.dirname(__file__))
from video_action_captioning_mapper import VideoActionCaptioningMapper  # noqa: E402


@OPERATORS.register_module("video_hawor_megasam_combined_mapper")
class VideoHaWorMegaSaMCombinedMapper(Mapper):
    """Run HaWoR and MegaSaM sequentially in one GPU actor."""

    _accelerator = "cuda"

    def __init__(
            self,
            hawor_model_path: str,
            hawor_config_path: str,
            hawor_detector_path: str,
            mano_right_path: str,
            mano_left_path: str,
            camera_calibration_field: str = MetaKeys.camera_calibration_moge_tags,
            hawor_tag_field: str = MetaKeys.hand_reconstruction_hawor_tags,
            frame_field: str = MetaKeys.video_frames,
            megasam_tag_field: str = MetaKeys.video_camera_pose_tags,
            megasam_save_dir: str | None = None,
            *args: Any,
            **kwargs: Any,
    ) -> None:
        super().__init__(*args, **kwargs)
        common = {
            "batch_mode": True,
            "skip_op_error": kwargs.get("skip_op_error", False),
        }
        self._hawor_kwargs = {
            "hawor_model_path": hawor_model_path,
            "hawor_config_path": hawor_config_path,
            "hawor_detector_path": hawor_detector_path,
            "mano_right_path": mano_right_path,
            "mano_left_path": mano_left_path,
            "camera_calibration_field": camera_calibration_field,
            "tag_field_name": hawor_tag_field,
            "frame_field": frame_field,
            "thresh": 0.2,
            **common,
        }
        self._megasam_kwargs = {
            "tag_field_name": megasam_tag_field,
            "camera_calibration_field": camera_calibration_field,
            "frame_field": frame_field,
            "max_frames": 1000,
            "droid_buffer": 1024,
            "save_dir": megasam_save_dir,
            **common,
        }
        self._hawor_op: VideoHandReconstructionHaworMapper | None = None
        self._megasam_op: VideoCameraPoseMegaSaMMapper | None = None

    def process_single(
            self,
            sample: dict[str, Any] | None = None,
            rank: int | None = None,
    ) -> dict[str, Any]:
        from loguru import logger

        if sample is None:
            raise ValueError("sample must not be None")
        if self._hawor_op is None:
            self._hawor_op = VideoHandReconstructionHaworMapper(
                **self._hawor_kwargs
            )
        if self._megasam_op is None:
            self._megasam_op = VideoCameraPoseMegaSaMMapper(
                **self._megasam_kwargs
            )

        sample = self._hawor_op.process_single(sample, rank=rank)
        try:
            return self._megasam_op.process_single(sample, rank=rank)
        except Exception as error:
            # Preserve HaWoR output if MegaSaM fails so later stages can continue.
            logger.exception(f"MegaSaM failed; HaWoR result is preserved: {error}")
            sample.setdefault(Fields.meta, {})
            frame_field = self._hawor_kwargs["frame_field"]
            tag_field = self._megasam_kwargs["tag_field_name"]
            sample[Fields.meta][tag_field] = [
                {} for _ in range(max(1, len(sample.get(frame_field, []))))
            ]
            return sample


@dataclass(frozen=True)
class Paths:
    root: Path
    frames: Path
    moge_arrays: Path
    megasam_arrays: Path
    lerobot: Path
    processed_json: Path
    processed_parquet: Path


@dataclass(frozen=True)
class Models:
    moge: Path
    hawor: Path
    hawor_config: Path
    hawor_detector: Path
    mano_right: Path
    mano_left: Path


def validate_models(root: Path) -> Models:
    """Require moge/, hawor/, and mano/ under MODEL_ROOT."""
    models = Models(
        moge=root / "moge/model.pt",
        hawor=root / "hawor/hawor.ckpt",
        hawor_config=root / "hawor/model_config.yaml",
        hawor_detector=root / "hawor/detector.pt",
        mano_right=root / "mano/MANO_RIGHT.pkl",
        mano_left=root / "mano/MANO_LEFT.pkl",
    )
    missing = [str(path) for path in models.__dict__.values() if not path.is_file()]
    if missing:
        raise FileNotFoundError(f"model files are missing: {missing}")
    return models


def prepare_paths() -> Paths:
    input_dir = Path(INPUT_DIR)
    output_root = Path(OUTPUT_ROOT)
    model_root = Path(MODEL_ROOT)
    for name, path in (
            ("INPUT_DIR", input_dir),
            ("OUTPUT_ROOT", output_root),
            ("MODEL_ROOT", model_root),
    ):
        if not path.is_absolute():
            raise ValueError(f"{name} must be an absolute shared-storage path")
    if not input_dir.is_dir():
        raise FileNotFoundError(f"INPUT_DIR does not exist: {input_dir}")
    if not model_root.is_dir():
        raise FileNotFoundError(f"MODEL_ROOT does not exist: {model_root}")

    run_id = time.strftime("v12-%Y%m%dT%H%M%SZ", time.gmtime())
    root = output_root / run_id
    root.mkdir(parents=True, exist_ok=False)
    return Paths(
        root=root,
        frames=root / "frames",
        moge_arrays=root / "moge_arrays",
        megasam_arrays=root / "megasam_arrays",
        lerobot=root / "lerobot_dataset",
        processed_json=root / "processed.jsonl",
        processed_parquet=root / "processed.parquet",
    )


def load_samples() -> list[dict[str, Any]]:
    videos = sorted(path for path in Path(INPUT_DIR).rglob("*.mp4") if path.is_file())
    if LIMIT > 0:
        videos = videos[:LIMIT]
    if not videos:
        raise RuntimeError(f"no MP4 files found under {INPUT_DIR}")
    return [
        {VIDEO_KEY: [str(path)], "text": TASK_TEXT, Fields.meta: {}}
        for path in videos
    ]


def run() -> None:
    import data_juicer

    if getattr(data_juicer, "__version__", "unknown") != EXPECTED_DATA_JUICER_VERSION:
        raise RuntimeError(
            f"Data-Juicer must be {EXPECTED_DATA_JUICER_VERSION}, got "
            f"{getattr(data_juicer, '__version__', 'unknown')}"
        )
    if LIMIT < 0 or FRAME_NUM < 1 or LEROBOT_FPS < 1:
        raise ValueError("LIMIT must be >= 0; FRAME_NUM and LEROBOT_FPS must be > 0")

    # Validate models before creating output directories to avoid partial output.
    models = validate_models(Path(MODEL_ROOT))
    samples = load_samples()
    paths = prepare_paths()

    worker_env = {
        "PYTHONUNBUFFERED": "1",
        "PYTHONDONTWRITEBYTECODE": "1",
        "HF_HUB_OFFLINE": os.environ.get("HF_HUB_OFFLINE", "1"),
        "TRANSFORMERS_OFFLINE": os.environ.get("TRANSFORMERS_OFFLINE", "1"),
        "VLA_FRAME_DIR_UNIQUE": "1",
    }
    for name in ("OPENAI_API_KEY", "OPENAI_ADMIN_KEY"):
        if os.environ.get(name):
            worker_env[name] = os.environ[name]

    ray.init(address="auto", runtime_env={"env_vars": worker_env})

    # Data-Juicer nested meta fields contain objects not representable by Arrow types.
    from ray.data import DataContext

    DataContext.get_current().enable_fallback_to_arrow_object_ext_type = True

    moge_pool = ActorPoolStrategy(
        min_size=MOGE_ACTOR_MIN,
        max_size=MOGE_ACTOR_MAX,
    )
    combined_pool = ActorPoolStrategy(
        min_size=COMBINED_ACTOR_MIN,
        max_size=COMBINED_ACTOR_MAX,
    )

    ds = ray.data.from_items(samples)
    ds = ds.map_batches(
        VideoExtractFramesMapper,
        fn_constructor_kwargs={
            "frame_sampling_method": "uniform",
            "frame_num": FRAME_NUM,
            "video_backend": "ffmpeg",
            "output_format": "path",
            "frame_dir": str(paths.frames),
            "frame_field": MetaKeys.video_frames,
            "legacy_split_by_text_token": False,
            "batch_mode": True,
            "skip_op_error": False,
            "video_key": VIDEO_KEY,
        },
        batch_size=1,
        num_cpus=CPU_PER_TASK,
        batch_format="pyarrow",
    )
    ds = ds.map_batches(
        VideoCameraCalibrationMogeMapper,
        fn_constructor_kwargs={
            "model_path": str(models.moge),
            "tag_field_name": MetaKeys.camera_calibration_moge_tags,
            "frame_field": MetaKeys.video_frames,
            "output_depth": True,
            "output_points": False,
            "output_mask": False,
            "save_dir": str(paths.moge_arrays),
            "batch_mode": True,
            "skip_op_error": False,
        },
        batch_size=GPU_BATCH_SIZE,
        num_gpus=MOGE_GPUS,
        batch_format="pyarrow",
        compute=moge_pool,
    )
    ds = ds.map_batches(
        VideoHaWorMegaSaMCombinedMapper,
        fn_constructor_kwargs={
            "hawor_model_path": str(models.hawor),
            "hawor_config_path": str(models.hawor_config),
            "hawor_detector_path": str(models.hawor_detector),
            "mano_right_path": str(models.mano_right),
            "mano_left_path": str(models.mano_left),
            "camera_calibration_field": MetaKeys.camera_calibration_moge_tags,
            "hawor_tag_field": MetaKeys.hand_reconstruction_hawor_tags,
            "megasam_tag_field": MetaKeys.video_camera_pose_tags,
            "frame_field": MetaKeys.video_frames,
            "megasam_save_dir": str(paths.megasam_arrays),
            "batch_mode": True,
            "skip_op_error": False,
        },
        batch_size=GPU_BATCH_SIZE,
        num_gpus=COMBINED_GPUS,
        batch_format="pyarrow",
        compute=combined_pool,
    )
    ds = ds.map_batches(
        VideoHandActionComputeMapper,
        fn_constructor_kwargs={
            "hand_reconstruction_field": MetaKeys.hand_reconstruction_hawor_tags,
            "camera_pose_field": MetaKeys.video_camera_pose_tags,
            "tag_field_name": MetaKeys.hand_action_tags,
            "hand_type": "both",
            "batch_mode": True,
            "skip_op_error": False,
        },
        batch_size=1,
        num_cpus=CPU_PER_TASK,
        batch_format="pyarrow",
    )

    caption_kwargs: dict[str, Any] = {
        "api_or_hf_model": CAPTION_MODEL,
        "is_api_model": True,
        "hand_type": "both",
        "frame_field": MetaKeys.video_frames,
        "tag_field_name": "hand_action_caption",
        "batch_mode": True,
        "skip_op_error": False,
    }
    if CAPTION_API_ENDPOINT:
        caption_kwargs["api_endpoint"] = CAPTION_API_ENDPOINT
    ds = ds.map_batches(
        VideoActionCaptioningMapper,
        fn_constructor_kwargs=caption_kwargs,
        batch_size=1,
        num_cpus=CPU_PER_TASK,
        batch_format="pyarrow",
    )
    ds = ds.map_batches(
        ExportToLeRobotMapper,
        fn_constructor_kwargs={
            "output_dir": str(paths.lerobot),
            "hand_action_field": MetaKeys.hand_action_tags,
            "frame_field": MetaKeys.video_frames,
            "video_key": VIDEO_KEY,
            "task_description_key": "text",
            "fps": LEROBOT_FPS,
            "robot_type": "egodex_hand",
            "batch_mode": True,
            "skip_op_error": False,
        },
        batch_size=1,
        num_cpus=CPU_PER_TASK,
        batch_format="pyarrow",
    )

    result = ds.materialize()
    output_rows = int(result.count())
    result.write_json(str(paths.processed_json), force_ascii=False)
    result.write_parquet(str(paths.processed_parquet))
    ExportToLeRobotMapper.finalize_dataset(
        output_dir=str(paths.lerobot),
        fps=LEROBOT_FPS,
        robot_type="egodex_hand",
    )

    print(
        json.dumps(
            {
                "status": "succeeded",
                "input_rows": len(samples),
                "output_rows": output_rows,
                "output_dir": str(paths.root),
                "lerobot_dataset": str(paths.lerobot),
            },
            ensure_ascii=False,
            indent=2,
        ),
        flush=True,
    )
    ray.shutdown()


if __name__ == "__main__":
    run()
