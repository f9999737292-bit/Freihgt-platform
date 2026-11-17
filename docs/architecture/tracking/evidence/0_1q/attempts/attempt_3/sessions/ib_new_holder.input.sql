-- step 120 [new holder issues while gate closed] sent 2026-11-17T16:14:04.997030+00:00
SELECT proof.issue_ticket('ns-main', 'req-b-1', encode(sha256(convert_to('0.1q:req-b-1', 'UTF8')), 'hex'), 2);
