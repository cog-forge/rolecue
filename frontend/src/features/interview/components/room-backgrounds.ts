import {
  environmentScenes,
  type EnvironmentSceneId,
} from "@/components/avatar-viewer/environment/scenes";

const scene = (
  id: `${EnvironmentSceneId}-3d`,
  sceneId: EnvironmentSceneId,
) => ({
  id,
  scene: sceneId,
  // The still shows while the 3D room loads, and stays when this device can't render it.
  image: environmentScenes[sceneId].still,
  thumbnail: environmentScenes[sceneId].thumbnail,
});

const photo = (id: "office" | "balcony" | "cafe") => ({
  id,
  scene: null,
  image: `/images/interview-room/${id}.webp`,
  thumbnail: `/images/interview-room/${id}-thumb.webp`,
});

/** 3D rooms first; the generated photos stay as the lightweight choice. Names live in the catalog. */
export const roomBackgrounds = [
  scene("cafe-3d", "cafe"),
  scene("office-3d", "office"),
  scene("meeting-3d", "meeting"),
  photo("office"),
  photo("balcony"),
  photo("cafe"),
] as const;

export type RoomBackground = (typeof roomBackgrounds)[number];
export type RoomBackgroundId = RoomBackground["id"];

export const defaultRoomBackground: RoomBackgroundId = "cafe-3d";
