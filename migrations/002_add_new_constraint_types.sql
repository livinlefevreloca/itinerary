-- +migrate Up

-- Example: Adding new constraint types
-- IDs 9-10 for new constraint types
INSERT OR IGNORE INTO constraint_types (id, name) VALUES
    (9, 'minHealthyInstances'),
    (10, 'requiredResources');

-- IDs 7-8 for new action types
INSERT OR IGNORE INTO action_types (id, name) VALUES
    (7, 'sendEmail'),
    (8, 'createTicket');
