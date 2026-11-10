-- Old provider transport errors could include the request's API key. Explanations are derived
-- diagnostics, not identity/history: clear affected traces before subsequent ordinary backups.
UPDATE items SET match_info = ''
WHERE lower(match_info) LIKE '%api_key%'
   OR lower(match_info) LIKE '%apikey%'
   OR lower(match_info) LIKE '%access_token=%';
