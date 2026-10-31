DROP TRIGGER IF EXISTS accountability_circle_members_immutable ON accountability_circle_members;
DROP TRIGGER IF EXISTS accountability_circles_immutable ON accountability_circles;
DROP FUNCTION IF EXISTS reject_accountability_circle_mutation();
DROP TRIGGER IF EXISTS accountability_circle_member_validation ON accountability_circle_members;
DROP FUNCTION IF EXISTS validate_accountability_circle_member();
DROP TABLE IF EXISTS accountability_circle_members;
DROP TABLE IF EXISTS accountability_circles;
DROP TABLE IF EXISTS accountability_invitations;
