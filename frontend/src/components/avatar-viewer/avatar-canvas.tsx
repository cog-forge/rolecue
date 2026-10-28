"use client";

import { useEffect, useMemo, useRef, useState, type ComponentRef } from "react";
import { Canvas, useFrame, useThree } from "@react-three/fiber";
import { ContactShadows, OrbitControls } from "@react-three/drei";
import { VRMUtils, type VRM } from "@pixiv/three-vrm";
import { Box3, Vector3 } from "three";
import { loadVrm, VrmLoadError, type VrmLoadErrorKind } from "./load-vrm";
import { poseIdle } from "./pose";
import type { ModelAccess } from "./avatar-viewer";

export type CanvasStatus =
  | { state: "loading" }
  | { state: "ready" }
  | { state: "error"; kind: VrmLoadErrorKind }
  | { state: "fallback" };

// Loaded only through next/dynamic (ssr: false), so three and three-vrm stay out of route bundles.
export default function AvatarCanvas({
  access,
  resetSignal,
  onStatus,
}: {
  access: ModelAccess;
  resetSignal: number;
  onStatus: (status: CanvasStatus) => void;
}) {
  const [vrm, setVrm] = useState<VRM | null>(null);

  useEffect(() => {
    const abort = new AbortController();
    let loaded: VRM | null = null;
    onStatus({ state: "loading" });
    loadVrm(access, abort.signal)
      .then((result) => {
        loaded = result;
        poseIdle(result);
        setVrm(result);
        onStatus({ state: "ready" });
      })
      .catch((error: unknown) => {
        if (abort.signal.aborted) return;
        onStatus({
          state: "error",
          kind: error instanceof VrmLoadError ? error.kind : "invalid",
        });
      });
    return () => {
      abort.abort();
      setVrm(null);
      if (loaded) VRMUtils.deepDispose(loaded.scene);
    };
  }, [access, onStatus]);

  return (
    <Canvas
      frameloop="demand"
      dpr={[1, 2]}
      camera={{ fov: 30, near: 0.05, far: 50, position: [0, 1.3, 4] }}
      gl={{ antialias: true, alpha: true }}
      onCreated={({ gl }) => {
        gl.domElement.addEventListener("webglcontextlost", (event) => {
          event.preventDefault();
          onStatus({ state: "fallback" });
        });
      }}
      fallback={null}
    >
      <hemisphereLight args={["#ffffff", "#8a8a8a", 1.6]} />
      <directionalLight position={[1.5, 3, 2.5]} intensity={1.8} />
      {vrm && <Scene vrm={vrm} resetSignal={resetSignal} />}
    </Canvas>
  );
}

function Scene({ vrm, resetSignal }: { vrm: VRM; resetSignal: number }) {
  const controls = useRef<ComponentRef<typeof OrbitControls>>(null);
  const camera = useThree((state) => state.camera);
  const invalidate = useThree((state) => state.invalidate);
  // The model's bounds, measured once per model; the camera and the reset button frame it whole.
  const frame = useMemo(() => {
    const box = new Box3().setFromObject(vrm.scene);
    return {
      target: box.getCenter(new Vector3()),
      size: Math.max(box.getSize(new Vector3()).y, 0.5),
      floor: box.min.y,
    };
  }, [vrm]);

  useEffect(() => {
    const { target, size } = frame;
    camera.position.set(
      target.x,
      target.y + size * 0.05,
      target.z + size * 2.1,
    );
    camera.lookAt(target);
    if (controls.current) {
      controls.current.target.copy(target);
      controls.current.update();
      controls.current.saveState();
    }
    invalidate();
  }, [frame, camera, invalidate]);

  useEffect(() => {
    if (resetSignal === 0) return;
    controls.current?.reset();
    invalidate();
  }, [resetSignal, invalidate]);

  // Spring bones and constraints follow the pose; capped so a delayed demand frame doesn't jump.
  useFrame((_, delta) => vrm.update(Math.min(delta, 1 / 30)));

  return (
    <>
      <primitive object={vrm.scene} />
      {/* Grounds the model on the backdrop's floor; one frame, since the model stands still. */}
      <ContactShadows
        position={[frame.target.x, frame.floor + 0.002, frame.target.z]}
        scale={frame.size * 1.4}
        opacity={0.38}
        blur={2.2}
        far={frame.size * 0.6}
        frames={1}
      />
      <OrbitControls
        ref={controls}
        makeDefault
        enablePan={false}
        // Damping keeps rendering after the drag ends; with frameloop="demand" that would burn
        // frames on slow (software WebGL) devices for no visible benefit.
        enableDamping={false}
        minDistance={frame.size * 0.6}
        maxDistance={frame.size * 4}
      />
    </>
  );
}
