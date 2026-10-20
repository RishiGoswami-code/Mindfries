import type { AvatarState } from "./Avatar2D";

/**
 * What the avatar card needs from whatever drives it: the simulated avatar
 * (design review, `useMockAvatar`) or the real one (`useAvatarLive`, Gemini
 * Live with the panel's own camera, screen and microphone).
 */
export interface AvatarController {
  state: AvatarState;
  /** 0 to 1: how loud it is speaking, for the mouth. */
  level: number;
  muted: boolean;
  talk: () => void;
  toggleMute: () => void;
  /** Plain words under the avatar when something needs saying (a problem, "starts when the clock does"). */
  note?: string;
  /** False when the avatar can't start (no key, off for this assessment, clock not running). */
  available?: boolean;
}
