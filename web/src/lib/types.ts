// Shared types for Frameline Studio.

// --- Tool option vocabularies (mirror the Go contracts) --------------------

export const CANVAS_PRESETS = [
  { value: '1920x1080', label: '16:9 Landscape' },
  { value: '1080x1920', label: '9:16 Reel' },
  { value: '1080x1080', label: '1:1 Square' },
] as const;

export const MOTION_OPTIONS = [
  { value: 'ken_burns', label: 'Ken Burns' },
  { value: 'slow_push', label: 'Slow Push' },
  { value: 'pan_left', label: 'Pan Left' },
  { value: 'pan_right', label: 'Pan Right' },
  { value: 'none', label: 'None' },
] as const;

export const TRANSITION_OPTIONS = [
  { value: 'fade', label: 'Fade' },
  { value: 'film_dissolve', label: 'Dissolve' },
  { value: 'wipe', label: 'Wipe' },
  { value: 'slide', label: 'Slide' },
  { value: 'zoom_blur', label: 'Zoom' },
  { value: 'random_safe', label: 'Random' },
  { value: 'none', label: 'Cut' },
] as const;

export const GRADE_OPTIONS = [
  { value: 'neutral', label: 'Neutral' },
  { value: 'warm', label: 'Warm' },
  { value: 'cinematic', label: 'Cinematic' },
  { value: 'vintage', label: 'Vintage' },
  { value: 'high_contrast', label: 'High Contrast' },
] as const;

// A TemplatePreset mirrors the Go templates.Preset registry (internal/templates).
// Picking a template pre-fills the chips with these values; the user can then
// tweak any of them. Keep these in sync with internal/templates/templates.go.
export interface TemplatePreset {
  canvas: string;
  fps: number;
  motion: string;
  transition: string;
  transitionSeconds: number;
  grade: string;
  secondsPerImage: number;
}

// TEMPLATES are the V6 cinematic presets. "none" carries no preset (Custom).
export const TEMPLATES: ReadonlyArray<{
  value: string;
  label: string;
  hint: string;
  preset?: TemplatePreset;
}> = [
  { value: 'none', label: 'Custom', hint: 'Set every control yourself.' },
  {
    value: 'wedding_reel',
    label: 'Wedding Reel',
    hint: 'Warm, unhurried — gentle Ken Burns, soft dissolves.',
    preset: { canvas: '1920x1080', fps: 30, motion: 'ken_burns', transition: 'film_dissolve', transitionSeconds: 1.2, grade: 'warm', secondsPerImage: 5 },
  },
  {
    value: 'product_launch',
    label: 'Product Launch',
    hint: 'Punchy 9:16 — slow push, quick slides, high contrast.',
    preset: { canvas: '1080x1920', fps: 30, motion: 'slow_push', transition: 'slide', transitionSeconds: 0.6, grade: 'high_contrast', secondsPerImage: 3 },
  },
  {
    value: 'memory_montage',
    label: 'Memory Montage',
    hint: 'Nostalgic — easy Ken Burns, crossfades, vintage grade.',
    preset: { canvas: '1920x1080', fps: 30, motion: 'ken_burns', transition: 'fade', transitionSeconds: 1.0, grade: 'vintage', secondsPerImage: 4 },
  },
  {
    value: 'travel_diary',
    label: 'Travel Diary',
    hint: 'Breezy — lateral pans, directional wipes, cinematic grade.',
    preset: { canvas: '1920x1080', fps: 30, motion: 'pan_right', transition: 'wipe', transitionSeconds: 0.8, grade: 'cinematic', secondsPerImage: 4 },
  },
] as const;

export const CAPTION_POSITIONS = [
  { value: 'lower_third', label: 'Lower third' },
  { value: 'top', label: 'Top' },
  { value: 'center', label: 'Center' },
] as const;

// UICaption is the editor row; it maps to the contract Caption on render.
export interface UICaption {
  id: string;
  text: string;
  start: number;
  end: number;
  position: string;
}

// --- Editor state ----------------------------------------------------------

export type ClipStatus = 'uploading' | 'ready' | 'error';

// A Clip is one image in the filmstrip. `previewUrl` is a local blob/data URL
// for in-iframe display; `path` is the server-side path (set once ingested or
// chosen via browse) and is what the render tool consumes.
export interface Clip {
  id: string;
  name: string;
  path?: string;
  previewUrl?: string;
  status: ClipStatus;
  error?: string;
}

export interface AudioBed {
  name: string;
  path: string;
  previewUrl?: string;
}

// --- Tool payloads (mirror Go contracts) -----------------------------------

export interface RenderOutput {
  output_path: string;
  job_id: string;
  duration_sec: number;
  width: number;
  height: number;
  size_bytes: number;
  command: string;
}

export interface CinematicOutput {
  render: RenderOutput;
  image_count: number;
  per_image_seconds: number;
  filter_complex: string;
  warnings?: string[];
}

export interface CinematicInput {
  images: string[];
  output_path?: string;
  template?: string;
  canvas?: string;
  fps?: number;
  duration_per_image?: number;
  total_duration?: number;
  transition_style?: string;
  transition_seconds?: number;
  motion_style?: string;
  color_grade?: string;
  background_audio?: string;
  audio_fade_in_seconds?: number;
  audio_fade_out_seconds?: number;
  captions?: Array<{ text: string; start_seconds: number; end_seconds: number; position: string }>;
}

export interface IngestMediaOutput {
  path: string;
  name: string;
  kind: string;
  size_bytes: number;
}

export interface ReadMediaOutput {
  data_uri?: string;
  mime: string;
  size_bytes: number;
  truncated?: boolean;
}

export interface MediaItem {
  path: string;
  name: string;
  kind: string;
  ext: string;
  size_bytes: number;
}

export interface ListMediaOutput {
  items: MediaItem[];
  roots: string[];
  truncated?: boolean;
}

let counter = 0;
export function nextId(): string {
  counter += 1;
  return `clip-${counter}`;
}

/** Reads a File into a base64 string (no data: prefix) for ingest_media. */
export function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error('read failed'));
    reader.onload = () => {
      const result = String(reader.result);
      const comma = result.indexOf(',');
      resolve(comma >= 0 ? result.slice(comma + 1) : result);
    };
    reader.readAsDataURL(file);
  });
}
