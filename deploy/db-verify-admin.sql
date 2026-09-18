-- 后台管理库（teyvat_admin）导入后的校验
-- 用法： mysql --host=<RDS地址> --user=teyvat_ops -p --default-character-set=utf8mb4 --database=teyvat_admin < deploy/db-verify-admin.sql
-- 判读：只报不一致的行；没有任何输出 = 全部对得上
SELECT tbl, expected, actual, (actual - expected) AS diff FROM (
             SELECT 'users'              AS tbl, 1 AS expected, COUNT(*) AS actual FROM users
    UNION ALL SELECT 'managed_projects',       2, COUNT(*) FROM managed_projects
    UNION ALL SELECT 'user_sessions',          5, COUNT(*) FROM user_sessions
    UNION ALL SELECT 'posts',                  0, COUNT(*) FROM posts
    UNION ALL SELECT 'project_notifications',  0, COUNT(*) FROM project_notifications
    UNION ALL SELECT 'phone_verification_codes', 0, COUNT(*) FROM phone_verification_codes
) AS t WHERE actual <> expected;
-- 关键内容抽查：管理员账号与两个已注册项目
SELECT 'users' AS what, id, username, permission_level FROM users
UNION ALL SELECT 'project', id, code, NULL FROM managed_projects;
-- 结构一致性：6 张表应全部是 utf8mb4_unicode_ci
SELECT table_name, table_collation FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_collation <> 'utf8mb4_unicode_ci';