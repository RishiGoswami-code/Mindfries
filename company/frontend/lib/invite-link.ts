// An invitation's own link on the candidate portal: /invite/<token>
// (migration 0022). It opens that one assessment for the invited email only
// — the candidate still signs in as that address — so it is safe to show to
// the team and to send by any channel.

export function inviteLink(token: string | null | undefined): string | null {
  const portal = process.env.CANDIDATE_PORTAL_URL?.replace(/\/+$/, "");
  if (!portal || !token) return null;
  return `${portal}/invite/${token}`;
}
