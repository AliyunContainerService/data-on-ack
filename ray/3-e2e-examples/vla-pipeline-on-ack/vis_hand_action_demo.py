#!/usr/bin/env python
"""Verify hand-action annotations without depending on ``hawor_utils``.

The script still uses the HaWoR repository and MANO model files bundled or
mounted in the runtime environment, but all local ``hawor_utils`` glue code is
included here.

Usage:
    python vis_hand_action_demo_standalone.py \
        --data_path processed.parquet \
        --save_dir ./vis_action_verify \
        --model_root /data/oss/models
"""

import argparse
import fractions
import json
import os
import pickle
import sys
from contextlib import redirect_stdout

import cv2
import numpy as np
import pyarrow.parquet as pq
import torch

from data_juicer.utils.cache_utils import DATA_JUICER_ASSETS_CACHE
from data_juicer.utils.constant import Fields, MetaKeys


# ---------------------------------------------------------------------------
# Inlined hawor_utils
# ---------------------------------------------------------------------------

HAWOR_HOME = os.path.expanduser(
    os.environ.get(
        "HAWOR_HOME",
        os.path.join(DATA_JUICER_ASSETS_CACHE, "HaWoR"),
    )
)

if not os.path.isdir(HAWOR_HOME):
    raise FileNotFoundError(
        f"HaWoR repository not found: {HAWOR_HOME}. "
        "Set HAWOR_HOME to the HaWoR repository path."
    )
if HAWOR_HOME not in sys.path:
    sys.path.insert(0, HAWOR_HOME)

# HaWoR imports must happen after HAWOR_HOME is added to sys.path.
from hawor.utils.geometry import aa_to_rotmat  # noqa: E402
from lib.models.mano_wrapper import MANO  # noqa: E402


_MANO_MODEL_PATHS = {"right": None, "left": None}


def _existing_file(paths):
    for path in paths:
        if path:
            expanded = os.path.abspath(os.path.expanduser(path))
            if os.path.isfile(expanded):
                return expanded
    return None


def configure_mano_paths(
    model_root=None,
    mano_right_path=None,
    mano_left_path=None,
):
    """Resolve MANO files from CLI arguments, environment, or known mounts."""
    model_root = model_root or os.environ.get("MODEL_ROOT")
    model_roots = [model_root] if model_root else []
    model_roots.extend(["/data/oss/models", "/data/models"])

    right_candidates = [
        mano_right_path,
        os.environ.get("MANO_RIGHT_PATH"),
        *(os.path.join(root, "mano/MANO_RIGHT.pkl") for root in model_roots),
        os.path.join(HAWOR_HOME, "_DATA/data/mano/MANO_RIGHT.pkl"),
    ]
    left_candidates = [
        mano_left_path,
        os.environ.get("MANO_LEFT_PATH"),
        *(os.path.join(root, "mano/MANO_LEFT.pkl") for root in model_roots),
        os.path.join(HAWOR_HOME, "_DATA/data_left/mano_left/MANO_LEFT.pkl"),
    ]

    _MANO_MODEL_PATHS["right"] = _existing_file(right_candidates)
    _MANO_MODEL_PATHS["left"] = _existing_file(left_candidates)

    if _MANO_MODEL_PATHS["right"] is None:
        searched = "\n  - ".join(
            os.path.abspath(os.path.expanduser(path))
            for path in right_candidates
            if path
        )
        raise FileNotFoundError(
            "MANO_RIGHT.pkl was not found. Provide --model_root, "
            "--mano_right_path, MODEL_ROOT, or MANO_RIGHT_PATH. "
            f"Searched:\n  - {searched}"
        )

    print(f"MANO right model: {_MANO_MODEL_PATHS['right']}")
    if _MANO_MODEL_PATHS["left"]:
        print(f"MANO left model: {_MANO_MODEL_PATHS['left']}")


def _mano_config(is_left=False):
    hand = "left" if is_left else "right"
    model_path = _MANO_MODEL_PATHS[hand]
    if model_path is None:
        option = "--mano_left_path" if is_left else "--mano_right_path"
        filename = "MANO_LEFT.pkl" if is_left else "MANO_RIGHT.pkl"
        raise FileNotFoundError(
            f"{filename} is required for {hand}-hand visualization; "
            f"provide {option} or --model_root."
        )

    data_dir = os.path.join(
        HAWOR_HOME,
        "_DATA/data_left" if is_left else "_DATA/data",
    )
    config = {
        "data_dir": data_dir,
        "model_path": model_path,
        "gender": "neutral",
        "num_hand_joints": 15,
        "create_body_pose": False,
    }
    if is_left:
        config["is_rhand"] = False
    return config


def _create_mano(is_left=False, use_cuda=None, fix_shapedirs=True):
    use_cuda = torch.cuda.is_available() if use_cuda is None else use_cuda
    if use_cuda and not torch.cuda.is_available():
        raise RuntimeError("CUDA was requested for MANO, but CUDA is unavailable")

    with open(os.devnull, "w") as sink, redirect_stdout(sink):
        mano = MANO(**_mano_config(is_left=is_left))

    if is_left and fix_shapedirs:
        # Work around the left-hand MANO shapedirs issue documented by smplx.
        mano.shapedirs[:, 0, :] *= -1

    device = torch.device("cuda" if use_cuda else "cpu")
    return mano.to(device), device


def get_mano_faces():
    mano, _ = _create_mano(is_left=False, use_cuda=False)
    return mano.faces


def _run_mano(
    trans,
    root_orient,
    hand_pose,
    betas,
    *,
    is_left=False,
    use_cuda=None,
    fix_shapedirs=True,
):
    if betas is None:
        raise ValueError("betas must not be None")

    mano, device = _create_mano(
        is_left=is_left,
        use_cuda=use_cuda,
        fix_shapedirs=fix_shapedirs,
    )
    batch_size, frame_count, _ = root_orient.shape
    num_joints = 15

    global_orient = aa_to_rotmat(
        root_orient.reshape(batch_size * frame_count, -1)
    ).view(batch_size * frame_count, 1, 3, 3)
    hand_pose_rotmat = aa_to_rotmat(
        hand_pose.reshape(batch_size * frame_count * num_joints, 3)
    ).view(batch_size * frame_count, num_joints, 3, 3)

    mano_output = mano(
        global_orient=global_orient.float().to(device),
        hand_pose=hand_pose_rotmat.float().to(device),
        betas=betas.reshape(batch_size * frame_count, -1).float().to(device),
        transl=trans.reshape(batch_size * frame_count, 3).float().to(device),
        pose2rot=False,
    )
    return {
        "joints": mano_output.joints.reshape(
            batch_size, frame_count, -1, 3
        ),
        "vertices": mano_output.vertices.reshape(
            batch_size, frame_count, -1, 3
        ),
    }


def run_mano(
    trans,
    root_orient,
    hand_pose,
    is_right=None,
    betas=None,
    use_cuda=None,
):
    del is_right
    return _run_mano(
        trans,
        root_orient,
        hand_pose,
        betas,
        is_left=False,
        use_cuda=use_cuda,
    )


def run_mano_left(
    trans,
    root_orient,
    hand_pose,
    is_right=None,
    betas=None,
    use_cuda=None,
    fix_shapedirs=True,
):
    del is_right
    return _run_mano(
        trans,
        root_orient,
        hand_pose,
        betas,
        is_left=True,
        use_cuda=use_cuda,
        fix_shapedirs=fix_shapedirs,
    )


# ---------------------------------------------------------------------------
# Visualization
# ---------------------------------------------------------------------------


def load_image(image_input):
    if isinstance(image_input, (str, bytes)):
        if isinstance(image_input, str):
            if not os.path.exists(image_input):
                raise ValueError(f"Error: File not found at {image_input}")
            img = cv2.imread(image_input)
        else:
            nparr = np.frombuffer(image_input, np.uint8)
            img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)

        if img is None:
            raise ValueError("Failed to decode image.")

        return img

    raise TypeError("Input must be a file path (str) or image bytes (bytes).")


def compute_hand_mesh(
    hand_transl,
    hand_orient,
    hand_pose,
    hand_betas,
    is_left=False,
):
    """Compute MANO hand mesh vertices, joints and faces."""
    transl = torch.tensor(hand_transl, dtype=torch.float32).unsqueeze(0)
    rot = torch.tensor(hand_orient, dtype=torch.float32).unsqueeze(0)
    pose = torch.tensor(hand_pose, dtype=torch.float32).unsqueeze(0)
    betas = torch.tensor(hand_betas, dtype=torch.float32).unsqueeze(0)

    mano_fn = run_mano_left if is_left else run_mano
    mano_out = mano_fn(transl, rot, pose, betas=betas)

    vertices = mano_out["vertices"][0].detach().cpu().numpy()
    joints = mano_out["joints"][0].detach().cpu().numpy()

    faces_base = get_mano_faces()
    faces_new = np.array(
        [
            [92, 38, 234],
            [234, 38, 239],
            [38, 122, 239],
            [239, 122, 279],
            [122, 118, 279],
            [279, 118, 215],
            [118, 117, 215],
            [215, 117, 214],
            [117, 119, 214],
            [214, 119, 121],
            [119, 120, 121],
            [121, 120, 78],
            [120, 108, 78],
            [78, 108, 79],
        ]
    )
    faces = np.concatenate([faces_base, faces_new], axis=0)
    if is_left:
        faces = faces[:, [0, 2, 1]]

    return vertices, joints, faces


def project_points_to_2d(points_3d, fov_x, width, height):
    """Project batch of 3D points to 2D pixel coords."""
    fx = width / (2.0 * np.tan(fov_x / 2.0))
    cx, cy = width / 2.0, height / 2.0
    z_safe = np.where(
        np.abs(points_3d[..., 2]) < 1e-6,
        1e-6,
        points_3d[..., 2],
    )
    u = fx * points_3d[..., 0] / z_safe + cx
    v = fx * points_3d[..., 1] / z_safe + cy
    return np.stack([u, v], axis=-1)


def draw_mesh_filled(frame, verts_2d, faces, color, alpha=0.3):
    """Draw filled semi-transparent mesh on frame."""
    overlay = frame.copy()
    height, width = frame.shape[:2]
    for face in faces:
        points = verts_2d[face].astype(np.int32)
        if np.any(points[:, 0] < -width) or np.any(points[:, 0] > 2 * width):
            continue
        if np.any(points[:, 1] < -height) or np.any(points[:, 1] > 2 * height):
            continue
        cv2.fillPoly(
            overlay,
            [points],
            color=color,
            lineType=cv2.LINE_AA,
        )
    cv2.addWeighted(overlay, alpha, frame, 1 - alpha, 0, frame)


def draw_mesh_wireframe(
    frame,
    verts_2d,
    faces,
    color,
    alpha=0.4,
    thickness=1,
):
    """Draw mesh wireframe on frame."""
    overlay = frame.copy()
    height, width = frame.shape[:2]
    for face in faces:
        points = verts_2d[face].astype(np.int32)
        if np.any(points[:, 0] < -width) or np.any(points[:, 0] > 2 * width):
            continue
        if np.any(points[:, 1] < -height) or np.any(points[:, 1] > 2 * height):
            continue
        cv2.polylines(
            overlay,
            [points],
            isClosed=True,
            color=color,
            thickness=thickness,
            lineType=cv2.LINE_AA,
        )
    cv2.addWeighted(overlay, alpha, frame, 1 - alpha, 0, frame)


MANO_SKELETON_BONES = [
    (0, 1),
    (1, 2),
    (2, 3),
    (3, 4),
    (0, 5),
    (5, 6),
    (6, 7),
    (7, 8),
    (0, 9),
    (9, 10),
    (10, 11),
    (11, 12),
    (0, 13),
    (13, 14),
    (14, 15),
    (15, 16),
    (0, 17),
    (17, 18),
    (18, 19),
    (19, 20),
]

FINGER_COLORS = {
    "thumb": (0, 255, 255),
    "index": (0, 0, 255),
    "middle": (0, 255, 0),
    "ring": (255, 165, 0),
    "pinky": (255, 0, 255),
}

BONE_FINGER_MAP = {
    0: "index",
    1: "index",
    2: "index",
    3: "index",
    4: "middle",
    5: "middle",
    6: "middle",
    7: "middle",
    8: "ring",
    9: "ring",
    10: "ring",
    11: "ring",
    12: "pinky",
    13: "pinky",
    14: "pinky",
    15: "pinky",
    16: "thumb",
    17: "thumb",
    18: "thumb",
    19: "thumb",
}


def draw_joints(
    frame,
    joints_2d,
    bone_color_override=None,
    joint_radius=4,
    bone_thickness=2,
    alpha=0.8,
):
    """Draw MANO hand joints and skeleton bones on frame."""
    overlay = frame.copy()
    height, width = frame.shape[:2]

    for bone_idx, (joint1, joint2) in enumerate(MANO_SKELETON_BONES):
        point1 = joints_2d[joint1].astype(np.int32)
        point2 = joints_2d[joint2].astype(np.int32)
        if (
            point1[0] < -width
            or point1[0] > 2 * width
            or point1[1] < -height
            or point1[1] > 2 * height
            or point2[0] < -width
            or point2[0] > 2 * width
            or point2[1] < -height
            or point2[1] > 2 * height
        ):
            continue
        color = (
            bone_color_override
            if bone_color_override is not None
            else FINGER_COLORS[BONE_FINGER_MAP[bone_idx]]
        )
        cv2.line(
            overlay,
            tuple(point1),
            tuple(point2),
            color,
            bone_thickness,
            cv2.LINE_AA,
        )

    for joint_idx in range(joints_2d.shape[0]):
        point = joints_2d[joint_idx].astype(np.int32)
        if (
            point[0] < -width
            or point[0] > 2 * width
            or point[1] < -height
            or point[1] > 2 * height
        ):
            continue
        radius = joint_radius + 2 if joint_idx == 0 else joint_radius
        cv2.circle(
            overlay,
            tuple(point),
            radius,
            (255, 255, 255),
            -1,
            cv2.LINE_AA,
        )
        cv2.circle(
            overlay,
            tuple(point),
            radius,
            (0, 0, 0),
            1,
            cv2.LINE_AA,
        )

    cv2.addWeighted(overlay, alpha, frame, 1 - alpha, 0, frame)


def world_to_camera(pos_world, cam_c2w):
    """Convert world position to camera space using cam_c2w."""
    cam_c2w = np.asarray(cam_c2w, dtype=np.float64)
    rotation = cam_c2w[:3, :3]
    translation = cam_c2w[:3, 3]
    return rotation.T @ (pos_world - translation)


def project_to_2d(pos_cam, fov_x, width, height):
    """Project camera-space position to 2D pixel coords."""
    fx = width / (2.0 * np.tan(fov_x / 2.0))
    cx, cy = width / 2.0, height / 2.0
    z_value = pos_cam[2] if abs(pos_cam[2]) > 1e-6 else 1e-6
    u_value = fx * pos_cam[0] / z_value + cx
    v_value = fx * pos_cam[1] / z_value + cy
    return np.array([u_value, v_value])


def draw_trajectory(frame, points_2d, color, thickness=2, dot_radius=4):
    """Draw trajectory line with dots."""
    for point_idx in range(1, len(points_2d)):
        point1 = tuple(points_2d[point_idx - 1].astype(int))
        point2 = tuple(points_2d[point_idx].astype(int))
        cv2.line(frame, point1, point2, color, thickness, cv2.LINE_AA)
    for point in points_2d:
        cv2.circle(
            frame,
            tuple(point.astype(int)),
            dot_radius,
            color,
            -1,
            cv2.LINE_AA,
        )


def draw_current_marker(frame, point, color, label=""):
    """Draw a highlighted marker for current position."""
    point_int = tuple(point.astype(int))
    cv2.circle(frame, point_int, 10, color, -1, cv2.LINE_AA)
    cv2.circle(frame, point_int, 12, (255, 255, 255), 2, cv2.LINE_AA)
    if label:
        cv2.putText(
            frame,
            label,
            (point_int[0] + 15, point_int[1] - 5),
            cv2.FONT_HERSHEY_SIMPLEX,
            0.55,
            color,
            2,
            cv2.LINE_AA,
        )


def draw_action_info(frame, hand_infos, frame_idx):
    """Draw per-hand state and action values on frame."""
    font = cv2.FONT_HERSHEY_SIMPLEX
    height, _ = frame.shape[:2]
    x_value = 10
    y_value = height - 22 * (len(hand_infos) * 3 + 1) - 10

    lines = [(f"Frame {frame_idx}", (255, 255, 255))]
    for info in hand_infos:
        hand_label = "R" if info["hand"] == "right" else "L"
        state = info["state"]
        action = info["action"]
        grip_text = "OPEN" if state[7] > 0 else "CLOSED"
        grip_color = (0, 255, 0) if state[7] > 0 else (0, 0, 255)

        lines.append(
            (
                f"[{hand_label}] Pos: [{state[0]:+.3f},{state[1]:+.3f},{state[2]:+.3f}]"
                f"  Rot: [{state[3]:+.2f},{state[4]:+.2f},{state[5]:+.2f}]",
                (255, 255, 255),
            )
        )
        lines.append(
            (
                f"[{hand_label}] dPos: [{action[0]:+.4f},{action[1]:+.4f},{action[2]:+.4f}]"
                f"  dRot: [{action[3]:+.3f},{action[4]:+.3f},{action[5]:+.3f}]",
                (200, 200, 200),
            )
        )
        lines.append(
            (
                f"[{hand_label}] Grip: {grip_text} ({state[7]:+.2f})",
                grip_color,
            )
        )

    for text, color in lines:
        (text_width, text_height), _ = cv2.getTextSize(
            text,
            font,
            0.45,
            1,
        )
        cv2.rectangle(
            frame,
            (x_value - 2, y_value - text_height - 2),
            (x_value + text_width + 4, y_value + 4),
            (0, 0, 0),
            -1,
        )
        cv2.putText(
            frame,
            text,
            (x_value, y_value),
            font,
            0.45,
            color,
            1,
            cv2.LINE_AA,
        )
        y_value += 22


def load_samples(data_path):
    if data_path.endswith((".json", ".jsonl")):
        with open(data_path, "r", encoding="utf-8") as data_file:
            return [json.loads(line) for line in data_file if line.strip()]
    if data_path.endswith(".pkl"):
        with open(data_path, "rb") as data_file:
            return pickle.load(data_file)
    if data_path.endswith(".parquet"):
        return pq.read_table(data_path).to_pylist()
    raise ValueError("data_path must end with .json, .jsonl, .pkl, or .parquet")


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Verify action annotations with 3D hand mesh and trajectory "
            "for both hands"
        ),
    )
    parser.add_argument(
        "--data_path",
        type=str,
        required=True,
        help="Path to data containing hand_action_tags",
    )
    parser.add_argument(
        "--save_dir",
        type=str,
        default="./vis_action_verify",
    )
    parser.add_argument(
        "--model_root",
        default=None,
        help=(
            "Model root containing mano/MANO_RIGHT.pkl and MANO_LEFT.pkl; "
            "defaults to MODEL_ROOT or common mount paths"
        ),
    )
    parser.add_argument(
        "--mano_right_path",
        default=None,
        help="Explicit path to MANO_RIGHT.pkl",
    )
    parser.add_argument(
        "--mano_left_path",
        default=None,
        help="Explicit path to MANO_LEFT.pkl",
    )
    parser.add_argument("--sample_idx", type=int, default=0)
    parser.add_argument("--video_idx", type=int, default=0)
    parser.add_argument("--fps", type=float, default=30)
    args = parser.parse_args()

    configure_mano_paths(
        model_root=args.model_root,
        mano_right_path=args.mano_right_path,
        mano_left_path=args.mano_left_path,
    )
    os.makedirs(args.save_dir, exist_ok=True)
    print(f"Loading: {args.data_path}")
    samples = load_samples(args.data_path)

    target_sample = samples[args.sample_idx]
    meta = target_sample[Fields.meta]
    if isinstance(meta, bytes):
        meta = pickle.loads(meta)

    frames = target_sample[MetaKeys.video_frames][args.video_idx]

    assert MetaKeys.hand_action_tags in meta, "Need hand_action_tags"
    assert MetaKeys.hand_reconstruction_hawor_tags in meta, "Need hawor tags"
    assert MetaKeys.video_camera_pose_tags in meta, "Need camera pose tags"

    action_tags = meta[MetaKeys.hand_action_tags][args.video_idx]
    hawor = meta[MetaKeys.hand_reconstruction_hawor_tags][args.video_idx]
    cam_pose = meta[MetaKeys.video_camera_pose_tags][args.video_idx]

    from data_juicer.utils.file_utils import load_numpy

    cam_c2w_all = load_numpy(cam_pose["cam_c2w"])
    fov_x = hawor["fov_x"]

    if "states" in action_tags:
        hand_type = action_tags.get("hand_type", "right")
        action_tags = {hand_type: action_tags}

    image = load_image(frames[0])
    image_height, image_width = image.shape[:2]

    hand_colors = {
        "right": {
            "gt": (0, 255, 0),
            "recon": (0, 165, 255),
            "mesh": (180, 120, 200),
        },
        "left": {
            "gt": (255, 255, 0),
            "recon": (255, 0, 255),
            "mesh": (200, 150, 50),
        },
    }
    hand_results = {}

    for hand_type in ["right", "left"]:
        hand_action = action_tags.get(hand_type, {})
        states_raw = hand_action.get("states", [])
        actions_raw = hand_action.get("actions", [])
        valid_ids = hand_action.get("valid_frame_ids", [])

        if len(states_raw) < 2:
            print(
                f"\n  {hand_type} hand: no action data from pipeline, "
                "skipping"
            )
            continue

        states = np.array(states_raw, dtype=np.float64)
        actions = np.array(actions_raw, dtype=np.float64)
        print(f"\n  === {hand_type.upper()} hand ===")
        is_left = hand_type == "left"

        if hand_type in hawor and isinstance(hawor[hand_type], dict):
            hand = hawor[hand_type]
            frame_ids = hand.get("frame_ids", [])
            hand_transl = hand.get("transl", [])
            hand_orient = hand.get("global_orient", [])
            hand_pose = hand.get("hand_pose", [])
            hand_betas = hand.get("betas", [])
        else:
            prefix = f"{hand_type}_"
            frame_ids = hawor.get(f"{prefix}frame_id_list", [])
            hand_transl = hawor.get(f"{prefix}transl_list", [])
            hand_orient = hawor.get(f"{prefix}global_orient_list", [])
            hand_pose = hawor.get(f"{prefix}hand_pose_list", [])
            hand_betas = hawor.get(f"{prefix}beta_list", [])

        joints_cam_raw = hand_action.get("joints_cam")
        if joints_cam_raw is not None and len(joints_cam_raw) > 0:
            joints_cam = np.array(joints_cam_raw, dtype=np.float64)
            joints_cam_id_map = {
                frame_id: idx for idx, frame_id in enumerate(valid_ids)
            }
            print(f"    joints_cam (from action_tags): {joints_cam.shape}")
        else:
            joints_cam = None
            joints_cam_id_map = {}

        if len(frame_ids) >= 2:
            mesh_verts, mesh_joints, mesh_faces = compute_hand_mesh(
                hand_transl,
                hand_orient,
                hand_pose,
                hand_betas,
                is_left=is_left,
            )
            print(f"    Mesh: vertices {mesh_verts.shape}")
        else:
            mesh_verts, mesh_joints, mesh_faces, frame_ids = (
                None,
                None,
                None,
                [],
            )

        frame_count = len(states)
        print(
            f"    States: {states.shape}, Actions: {actions.shape}, "
            f"Valid: {len(valid_ids)}"
        )
        mesh_id_map = {
            frame_id: idx for idx, frame_id in enumerate(frame_ids)
        }

        wrist_2d = []
        for frame_idx in range(frame_count):
            frame_id = valid_ids[frame_idx]
            pos_cam = world_to_camera(
                states[frame_idx, :3],
                cam_c2w_all[frame_id],
            )
            if joints_cam is not None and frame_id in joints_cam_id_map:
                joint_idx = joints_cam_id_map[frame_id]
                wrist_cam = joints_cam[joint_idx, 0, :]
                if frame_id in mesh_id_map:
                    mesh_idx = mesh_id_map[frame_id]
                    pos_cam = pos_cam + (
                        wrist_cam - np.asarray(hand_transl[mesh_idx])
                    )
            elif mesh_joints is not None and frame_id in mesh_id_map:
                mesh_idx = mesh_id_map[frame_id]
                wrist_cam = mesh_joints[mesh_idx, 0, :]
                pos_cam = pos_cam + (
                    wrist_cam - np.asarray(hand_transl[mesh_idx])
                )
            wrist_2d.append(
                project_to_2d(
                    pos_cam,
                    fov_x,
                    image_width,
                    image_height,
                )
            )

        hand_results[hand_type] = {
            "states": states,
            "actions": actions,
            "valid_ids": valid_ids,
            "wrist_2d": np.array(wrist_2d),
            "mesh_verts": mesh_verts,
            "mesh_joints": mesh_joints,
            "mesh_faces": mesh_faces,
            "mesh_id_map": mesh_id_map,
            "frame_ids": frame_ids,
            "joints_cam": joints_cam,
            "joints_cam_id_map": joints_cam_id_map,
        }

    if not hand_results:
        print("No valid hand data found!")
        return

    all_valid_frame_ids = sorted(
        set().union(
            *(
                set(result["valid_ids"])
                for result in hand_results.values()
            )
        )
    )

    print(f"\nRendering {len(all_valid_frame_ids)} frames...")
    frames_dir = os.path.join(args.save_dir, "frames")
    os.makedirs(frames_dir, exist_ok=True)
    output_frames = []

    for frame_seq, frame_id in enumerate(all_valid_frame_ids):
        frame = load_image(frames[frame_id])
        canvas = frame.copy()
        hand_infos = []

        for hand_type, result in hand_results.items():
            colors = hand_colors[hand_type]

            if (
                result["mesh_verts"] is not None
                and frame_id in result["mesh_id_map"]
            ):
                mesh_idx = result["mesh_id_map"][frame_id]
                verts_2d = project_points_to_2d(
                    result["mesh_verts"][mesh_idx],
                    fov_x,
                    image_width,
                    image_height,
                )
                draw_mesh_filled(
                    canvas,
                    verts_2d,
                    result["mesh_faces"],
                    colors["mesh"],
                    alpha=0.25,
                )
                draw_mesh_wireframe(
                    canvas,
                    verts_2d,
                    result["mesh_faces"],
                    colors["mesh"],
                    alpha=0.5,
                    thickness=1,
                )

            if (
                result["joints_cam"] is not None
                and frame_id in result["joints_cam_id_map"]
            ):
                joint_idx = result["joints_cam_id_map"][frame_id]
                joints_2d = project_points_to_2d(
                    result["joints_cam"][joint_idx],
                    fov_x,
                    image_width,
                    image_height,
                )
                draw_joints(
                    canvas,
                    joints_2d,
                    joint_radius=4,
                    bone_thickness=2,
                    alpha=0.85,
                )
            elif (
                result["mesh_joints"] is not None
                and frame_id in result["mesh_id_map"]
            ):
                mesh_idx = result["mesh_id_map"][frame_id]
                joints_2d = project_points_to_2d(
                    result["mesh_joints"][mesh_idx],
                    fov_x,
                    image_width,
                    image_height,
                )
                draw_joints(
                    canvas,
                    joints_2d,
                    joint_radius=4,
                    bone_thickness=2,
                    alpha=0.85,
                )

            if frame_id not in result["valid_ids"]:
                continue

            action_idx = result["valid_ids"].index(frame_id)
            trail = result["wrist_2d"][: action_idx + 1]
            if len(trail) >= 2:
                draw_trajectory(
                    canvas,
                    trail,
                    colors["gt"],
                    thickness=3,
                    dot_radius=4,
                )

            label_prefix = "R" if hand_type == "right" else "L"
            draw_current_marker(
                canvas,
                result["wrist_2d"][action_idx],
                colors["gt"],
                label_prefix,
            )
            hand_infos.append(
                {
                    "hand": hand_type,
                    "state": result["states"][action_idx],
                    "action": result["actions"][action_idx],
                }
            )

        if hand_infos:
            draw_action_info(canvas, hand_infos, frame_id)

        output_path = os.path.join(
            frames_dir,
            f"verify_{frame_seq:04d}.jpg",
        )
        cv2.imwrite(output_path, canvas)
        output_frames.append(canvas)

    if output_frames:
        import av

        video_path = os.path.join(args.save_dir, "action_verify.mp4")
        output_height, output_width = output_frames[0].shape[:2]
        fps_fraction = fractions.Fraction(args.fps).limit_denominator(10000)

        container = av.open(video_path, mode="w")
        stream = container.add_stream("libx264", rate=fps_fraction)
        stream.width = output_width
        stream.height = output_height
        stream.pix_fmt = "yuv420p"
        stream.options = {"crf": "18", "preset": "medium"}

        for frame_bgr in output_frames:
            frame_rgb = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2RGB)
            av_frame = av.VideoFrame.from_ndarray(frame_rgb, format="rgb24")
            for packet in stream.encode(av_frame):
                container.mux(packet)
        for packet in stream.encode():
            container.mux(packet)
        container.close()

        size_mb = os.path.getsize(video_path) / (1024 * 1024)
        print(f"\nVideo: {video_path} ({size_mb:.1f} MB)")

    print("Done!")


if __name__ == "__main__":
    main()
