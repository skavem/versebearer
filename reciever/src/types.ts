export type IShownVerse = {
  text: string;
  number: number;
  Book: {
    shortName: string;
  }
  Chapter: {
    number: number;
  }
}

export type IShownCouplet = {
  text: string;
}

export type IVisualStyle = {
  bgColor: string;
  bgOpacity: number;
  textColor: string;
  fontId: number | null;
  borderColor: string;
  borderWidth: number;
  borderRadius: number;
  borderStyle: string;
  padding: number;
  margin: number;
  textShadow: string;
}

// Theme-level always-on full-screen backdrop, independent of verse/couplet.
export type IBackdrop = {
  bgType: string;
  bgGradient: string;
  bgImageId: number | null;
}

export type IFont = {
  ID: number;
  name: string;
  mimeType: string;
  sizeBytes: number;
}