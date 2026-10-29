"use client";

import { useEffect, useMemo, useRef, useState, type ComponentRef } from "react";
import { Canvas, useFrame, useThree } from "@react-three/fiber";
import { ContactShadows, OrbitControls } from "@react-three/drei";
import { VRMUtils, type VRM } from "@pixiv/three-vrm";
import { Box3, Vector3, PerspectiveCamera } from "three";
import { loadVrm, VrmLoadError, type VrmLoadErrorKind } from "./load-vrm";
import { interviewFraming } from "./interview-framing";
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
  presentation = "preview",
  cameraView = "conversation",
  atmosphere = "warm",
  horizontalOffset = 0,
}: {
  access: ModelAccess;
  presentation?: "preview" | "interview";
  cameraView?: "conversation" | "close";
  atmosphere?: "warm" | "daylight" | "evening";
  horizontalOffset?: number;
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
      fallback={null}
    >
      <ContextLifecycle onStatus={onStatus} />
      <hemisphereLight
        args={[
          atmosphere === "warm" ? "#fff4e5" : "#eef4ff",
          "#77706a",
          atmosphere === "evening" ? 1.1 : 1.6,
        ]}
      />
      <directionalLight
        position={[1.5, 3, 2.5]}
        intensity={atmosphere === "daylight" ? 2.3 : 1.8}
      />
      {presentation === "interview" && (
        <directionalLight
          position={[-3, 2, -2]}
          color="#dce9ff"
          intensity={1.5}
        />
      )}
      {vrm && (
        <Scene
          vrm={vrm}
          resetSignal={resetSignal}
          immersive={presentation === "interview"}
          horizontalOffset={horizontalOffset}
          close={cameraView === "close"}
        />
      )}
    </Canvas>
  );
}

function ContextLifecycle({
  onStatus,
}: {
  onStatus: (status: CanvasStatus) => void;
}) {
  const canvas = useThree((state) => state.gl.domElement);
  useEffect(() => {
    const lost = (event: Event) => {
      event.preventDefault();
      onStatus({ state: "fallback" });
    };
    canvas.addEventListener("webglcontextlost", lost);
    return () => canvas.removeEventListener("webglcontextlost", lost);
  }, [canvas, onStatus]);
  return null;
}

function Scene({
  vrm,
  resetSignal,
  immersive,
  close,
  horizontalOffset,
}: {
  vrm: VRM;
  resetSignal: number;
  immersive: boolean;
  close: boolean;
  horizontalOffset: number;
}) {
  const controls = useRef<ComponentRef<typeof OrbitControls>>(null);
  const camera = useThree((state) => state.camera);
  const invalidate = useThree((state) => state.invalidate);
  const viewport = useThree((state) => state.size);
  // Measure once per model. Preview frames the full body; interview frames the upper body.
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
    const framing = interviewFraming(
      size,
      frame.floor,
      viewport.width,
      viewport.height,
      close,
      horizontalOffset,
    );
    const focus = immersive
      ? new Vector3(target.x, framing.targetY, target.z)
      : target;
    camera.position.set(
      target.x,
      immersive ? framing.eyeY : target.y + size * 0.05,
      target.z + (immersive ? framing.distance : size * 2.1),
    );
    if (camera instanceof PerspectiveCamera) {
      if (immersive)
        camera.setViewOffset(
          viewport.width,
          viewport.height,
          framing.offsetX,
          framing.offsetY,
          viewport.width,
          viewport.height,
        );
      else camera.clearViewOffset();
      camera.updateProjectionMatrix();
    }
    camera.lookAt(focus);
    if (controls.current) {
      controls.current.target.copy(focus);
      controls.current.update();
      controls.current.saveState();
    }
    invalidate();
  }, [
    frame,
    camera,
    invalidate,
    immersive,
    close,
    viewport.width,
    viewport.height,
    horizontalOffset,
  ]);

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
      {!immersive && (
        <ContactShadows
          position={[frame.target.x, frame.floor + 0.002, frame.target.z]}
          scale={frame.size * 1.4}
          opacity={0.38}
          blur={2.2}
          far={frame.size * 0.6}
          frames={1}
        />
      )}
      <OrbitControls
        ref={controls}
        makeDefault
        enablePan={false}
        enableZoom={!immersive}
        minAzimuthAngle={immersive ? -0.25 : -Infinity}
        maxAzimuthAngle={immersive ? 0.25 : Infinity}
        minPolarAngle={immersive ? Math.PI * 0.4 : 0}
        maxPolarAngle={immersive ? Math.PI * 0.55 : Math.PI}
        // Damping keeps rendering after the drag ends; with frameloop="demand" that would burn
        // frames on slow (software WebGL) devices for no visible benefit.
        enableDamping={false}
        minDistance={frame.size * 0.6}
        maxDistance={frame.size * 4}
      />
    </>
  );
}
