"use client";
import { memo, useEffect, useRef } from "react";
import { LoaderCircle, VideoOff } from "lucide-react";
import type { CameraSnapshot } from "../video/camera-capture";

export const RoomSelfView = memo(function RoomSelfView({
  camera,
  hideOnMobile = false,
}: {
  camera: CameraSnapshot;
  hideOnMobile?: boolean;
}) {
  const video = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    element.srcObject = camera.stream;
    return () => {
      element.srcObject = null;
    };
  }, [camera.stream]);
  return (
    <aside
      aria-label="Your camera"
      className={`absolute top-20 right-3 z-20 aspect-video w-32 overflow-visible rounded-2xl border border-white/20 bg-neutral-900/70 text-white shadow-2xl backdrop-blur-xl md:top-auto md:right-6 md:bottom-32 md:w-64 ${hideOnMobile ? "max-md:hidden" : ""}`}
    >
      {camera.stream ? (
        <video
          ref={video}
          aria-label="Your camera preview"
          autoPlay
          playsInline
          muted
          className="size-full -scale-x-100 rounded-[inherit] object-cover"
        />
      ) : (
        <div className="flex size-full flex-col items-center justify-center gap-2 px-3 pb-5 text-center">
          <span aria-hidden>
            {camera.status === "requesting" ? (
              <LoaderCircle className="size-4 animate-spin motion-reduce:animate-none" />
            ) : (
              <VideoOff className="size-5 text-white/60" />
            )}
          </span>
          <p className="text-[10px] text-white/65 md:text-xs">
            {camera.status === "requesting"
              ? "Waiting for camera…"
              : camera.error
                ? "Camera unavailable"
                : "Camera off"}
          </p>
        </div>
      )}
      <span className="absolute bottom-2 left-3 rounded-md bg-black/40 px-2 py-0.5 text-[10px] backdrop-blur-md md:bottom-3 md:text-xs">
        You
      </span>
    </aside>
  );
});
