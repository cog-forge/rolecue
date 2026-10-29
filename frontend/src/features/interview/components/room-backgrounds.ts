export const roomBackgrounds = [
  {
    id: "office",
    name: "Office",
    description: "A calm, sunlit studio",
    image: "/images/interview-room/office.webp",
    thumbnail: "/images/interview-room/office-thumb.webp",
  },
  {
    id: "balcony",
    name: "Balcony",
    description: "An open city terrace",
    image: "/images/interview-room/balcony.webp",
    thumbnail: "/images/interview-room/balcony-thumb.webp",
  },
  {
    id: "cafe",
    name: "Café",
    description: "A quiet café corner",
    image: "/images/interview-room/cafe.webp",
    thumbnail: "/images/interview-room/cafe-thumb.webp",
  },
] as const;
export type RoomBackgroundId = (typeof roomBackgrounds)[number]["id"];
