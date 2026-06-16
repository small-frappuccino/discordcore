UPDATE guild_configs
SET config_json = jsonb_set(
  config_json,
  '{stats}',
  '{"enabled": true, "update_interval_mins": 30, "channels": [{"label": "☆ total proxies ☆ : ", "channel_id": "1379653952639074374", "member_type": "all"}, {"label": "☆ bunny boosters ☆ : ", "role_id": "1375851519819124907", "channel_id": "1395994541324238848", "member_type": "humans"}, {"label": "☆ proxies ☆ : ", "channel_id": "1379653956376199228", "member_type": "humans"}, {"label": "☆ bangboos ☆ : ", "role_id": "1376361448942342164", "channel_id": "1379653960272449688", "member_type": "bots"}]}'::jsonb
)
WHERE guild_id = '1375650791251120179';
