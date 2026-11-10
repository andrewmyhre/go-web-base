// A client-rendered single-page app: the static adapter writes one
// index.html fallback and the browser renders every route. The signed-in
// user is per-browser state (a session cookie), so there is nothing to
// render ahead of time.
export const ssr = false;
export const prerender = false;
