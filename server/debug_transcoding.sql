-- 诊断转码状态的SQL查询
-- 请把 VIDEO_ID 和 RESOURCE_ID 替换成你的实际值

-- 1. 查看视频信息
SELECT id, title, status, created_at, updated_at
FROM video
WHERE id = VIDEO_ID;

-- 2. 查看该视频下的所有资源
SELECT id, vid, title, status, duration, created_at, updated_at
FROM resource
WHERE vid = VIDEO_ID;

-- 3. 查看该资源的所有清晰度文件
SELECT id, resource_id, quality, dir_name, created_at
FROM video_index_file
WHERE resource_id = RESOURCE_ID;

-- 4. 查看转码中的资源数量
SELECT COUNT(*) as processing_count
FROM resource
WHERE vid = VIDEO_ID AND status = 200;

-- 5. 查看成功的资源数量
SELECT COUNT(*) as success_count
FROM resource
WHERE vid = VIDEO_ID AND (status = 500 OR status = 0);

-- 状态码说明:
-- 0   = AUDIT_APPROVED (审核通过)
-- 100 = CREATED_VIDEO (创建视频)
-- 200 = VIDEO_PROCESSING (转码中)
-- 300 = SUBMIT_REVIEW (提交审核)
-- 500 = WAITING_REVIEW (等待审核)
-- 2000 = REVIEW_FAILED (审核失败)
-- 3000 = PROCESSING_FAIL (处理失败)
