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
  textShadow: string;
}

export type IFont = {
  ID: number;
  name: string;
  mimeType: string;
  sizeBytes: number;
}