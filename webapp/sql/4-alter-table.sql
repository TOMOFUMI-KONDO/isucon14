ALTER TABLE chairs
ADD COLUMN `available` BOOLEAN NOT NULL DEFAULT TRUE COMMENT '椅子が利用可能かどうか';

ALTER TABLE charis ADD INDEX `is_active_available_idx` (`is_active`, `available`);
