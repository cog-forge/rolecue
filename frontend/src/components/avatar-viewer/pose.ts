import type { VRM } from "@pixiv/three-vrm";

// A relaxed static pose instead of the export's T-pose. No animation: KAN-58/KAN-59 own motion.
export function poseIdle(vrm: VRM) {
  const bone = (
    name: Parameters<typeof vrm.humanoid.getNormalizedBoneNode>[0],
  ) => vrm.humanoid.getNormalizedBoneNode(name);
  const leftArm = bone("leftUpperArm");
  const rightArm = bone("rightUpperArm");
  if (leftArm) leftArm.rotation.z = -1.2;
  if (rightArm) rightArm.rotation.z = 1.2;
  const leftForearm = bone("leftLowerArm");
  const rightForearm = bone("rightLowerArm");
  if (leftForearm) leftForearm.rotation.z = -0.15;
  if (rightForearm) rightForearm.rotation.z = 0.15;
  vrm.update(0);
}
