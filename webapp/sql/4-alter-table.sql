ALTER TABLE chairs
ADD COLUMN `available` BOOLEAN NOT NULL DEFAULT TRUE COMMENT '椅子が利用可能かどうか';

ALTER TABLE chairs ADD INDEX `is_active_available_idx` (`is_active`, `available`);

ALTER TABLE rides
ADD COLUMN notified_completed TINYINT (1) NOT NULL DEFAULT 0 COMMENT '完了通知済みかどうか',
