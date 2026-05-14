The notification service is complete and builds cleanly. Here's a summary of what was created:

  ---
  services/notification-service — what's in it
  
  Event consumers (Pub/Sub push → /internal/pubsub)

  ┌──────────────────────────────┬──────────────┬──────────────────────────────┐
  │            Event             │ Source topic │            Action            │
  ├──────────────────────────────┼──────────────┼──────────────────────────────┤
  │ user.registered              │ user-events  │ Welcome email                │
  ├──────────────────────────────┼──────────────┼──────────────────────────────┤
  │ email.verification_requested │ user-events  │ Verification link email      │
  ├──────────────────────────────┼──────────────┼──────────────────────────────┤
  │ password.reset_requested     │ user-events  │ Password reset link email    │
  ├──────────────────────────────┼──────────────┼──────────────────────────────┤
  │ deck.clone_completed         │ deck-events  │ FCM push to all user devices │
  └──────────────────────────────┴──────────────┴──────────────────────────────┘
  
  gRPC / HTTP API (port 9095 / 8085)

  ┌───────────────────────┬──────────────────────────────────────────┬───────────────────────┐
  │        Method         │                   HTTP                   │      Description      │
  ├───────────────────────┼──────────────────────────────────────────┼───────────────────────┤
  │ RegisterDeviceToken   │ POST /v1/notifications/devices           │ Save user's FCM token │
  ├───────────────────────┼──────────────────────────────────────────┼───────────────────────┤
  │ UnregisterDeviceToken │ DELETE /v1/notifications/devices/{token} │ Remove FCM token      │
  └───────────────────────┴──────────────────────────────────────────┴───────────────────────┘

  Database tables

  - fcm_tokens — per-user device tokens (upserted on re-register)
  - notification_log — audit trail for every sent/failed notification

  Key design decisions

  - Graceful degradation: both SMTP and FCM are optional — the service starts and processes events even when neither is configured (noop implementations),
  making local dev easy
  - Pub/Sub subscriptions: notification-user-events-sub and notification-deck-events-sub added to deploy/pubsub-setup/init.sh
  - Firebase Admin SDK (firebase.google.com/go/v4) used for FCM; set FCM_PROJECT_ID + optionally FCM_CREDENTIALS_FILE to enable

  Setup checklist

  # 1. Run migrations against your DB
  cd services/notification-service && make migrateup

  # 2. Configure app.env (SMTP + FCM credentials)
  # 3. Regenerate proto if you change the .proto file
  make proto

✻ Sautéed for 16m 36s · 1 local agent still running

● Agent "Save project memory for notification service" completed

● Memory saved. The notification service is fully implemented and ready to use.