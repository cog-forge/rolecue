"""Avaturn T2 adapter; Blender and the VRM extension own import/export."""

import argparse
import json
from pathlib import Path
import sys
import tomllib

import addon_utils
import bpy


ADDON = "bl_ext.user_default.vrm"
VERSION = "4.10.2"
BONES = json.loads(Path(__file__).with_name("avaturn_profile.json").read_text())["bones"]

VOWELS = {"aa": "viseme_aa", "ih": "viseme_I", "ou": "viseme_U",
          "ee": "viseme_E", "oh": "viseme_O"}


def bind(expression, meshes, target):
    count = 0
    for mesh in meshes:
        keys = mesh.data.shape_keys
        if not keys or target not in keys.key_blocks:
            continue
        item = expression.morph_target_binds.add()
        item.node.mesh_object_name = mesh.name
        item.index = target
        item.weight = 1.0
        count += 1
    return count


def convert(args):
    module = addon_utils.enable(ADDON, default_set=True)
    if module is None:
        raise RuntimeError("VRM extension is not installed")
    manifest = tomllib.loads(Path(module.__file__).with_name("blender_manifest.toml").read_text())
    if manifest["version"] != VERSION:
        raise RuntimeError(f"VRM extension {VERSION} required")
    if bpy.app.version[:2] != (5, 2):
        raise RuntimeError("Blender 5.2 required")

    for obj in list(bpy.data.objects):
        bpy.data.objects.remove(obj, do_unlink=True)
    if bpy.ops.import_scene.gltf(filepath=args.input) != {"FINISHED"}:
        raise RuntimeError("GLB import failed")
    armatures = [obj for obj in bpy.data.objects if obj.type == "ARMATURE"]
    if len(armatures) != 1:
        raise ValueError("one Avaturn humanoid skeleton required")
    armature = armatures[0]
    meshes = [obj for obj in bpy.data.objects if obj.type == "MESH"]
    extension = armature.data.vrm_addon_extension
    extension.spec_version = "1.0"
    vrm = extension.vrm1
    for target, source in BONES.items():
        if source not in armature.data.bones:
            raise ValueError(f"unsupported Avaturn bone: {source}")
        getattr(vrm.humanoid.human_bones, target).node.bone_name = source

    metadata = json.loads(Path(args.metadata).read_text()) if args.metadata else {}
    meta = vrm.meta
    meta.vrm_name = metadata.get("name", "Avaturn T2 local verification")
    meta.version = "1.0"
    meta.authors.add().value = metadata.get("author", "Avaturn")
    meta.references.add().value = "https://avaturn.me/"
    # Usage policy is server-configured; the fallback is for local verification.
    meta.avatar_permission = "onlySeparatelyLicensedPerson"
    meta.commercial_usage = metadata.get("commercial_usage", "personalNonProfit")
    meta.credit_notation = "required"
    meta.allow_redistribution = False
    meta.modification = "allowModification"
    meta.other_license_url = metadata.get("license_url", "")

    expressions = vrm.expressions
    for target, source in VOWELS.items():
        if not bind(getattr(expressions.preset, target), meshes, source):
            raise ValueError(f"missing lip-sync morph: {source}")
    for target, source in (("blink_left", "eyeBlinkLeft"),
                           ("blink_right", "eyeBlinkRight"), ("blink", "eyesClosed")):
        bind(getattr(expressions.preset, target), meshes, source)
    # Preserve every source facial channel, including the full phoneme set.
    targets = sorted({key.name for mesh in meshes if mesh.data.shape_keys
                      for key in list(mesh.data.shape_keys.key_blocks)[1:]})
    for target in targets:
        custom = expressions.custom.add()
        custom.custom_name = target
        bind(custom, meshes, target)

    result = bpy.ops.export_scene.vrm(
        filepath=args.output, armature_object_name=armature.name,
        use_addon_preferences=False, export_invisibles=False,
        export_only_selections=False, export_gltf_animations=False,
        export_all_influences=True, enable_advanced_preferences=True,
        export_try_sparse_sk=False, ignore_warning=False,
    )
    if result != {"FINISHED"} or not Path(args.output).is_file():
        raise RuntimeError("VRM export failed")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--metadata")
    convert(parser.parse_args(sys.argv[sys.argv.index("--") + 1:]))
