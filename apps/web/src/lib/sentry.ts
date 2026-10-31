"use client";

// Sentry is opt-in via NEXT_PUBLIC_SENTRY_DSN. Events are scrubbed of
// user PII before sending; without a DSN this is a no-op.
export async function initSentry() {
  const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;
  if (!dsn) return;
  const Sentry = await import("@sentry/nextjs");
  Sentry.init({
    dsn,
    beforeSend(event) {
      if (event.user) {
        delete event.user.email;
        delete event.user.ip_address;
      }
      if (event.request?.headers) {
        delete event.request.headers["authorization"];
        delete event.request.headers["Authorization"];
        delete event.request.headers["cookie"];
        delete event.request.headers["Cookie"];
      }
      return event;
    },
  });
}
