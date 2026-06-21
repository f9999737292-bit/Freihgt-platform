INSERT INTO core.permissions (code, resource, action, description)
VALUES
    ('company.create', 'company', 'create', 'Create company'),
    ('company.read', 'company', 'read', 'Read company'),
    ('company.update', 'company', 'update', 'Update company'),

    ('user.invite', 'user', 'invite', 'Invite user'),
    ('user.read', 'user', 'read', 'Read user'),
    ('user.update', 'user', 'update', 'Update user'),

    ('transport_order.create', 'transport_order', 'create', 'Create transport order'),
    ('transport_order.read', 'transport_order', 'read', 'Read transport order'),
    ('transport_order.update', 'transport_order', 'update', 'Update transport order'),
    ('transport_order.submit', 'transport_order', 'submit', 'Submit transport order'),

    ('shipment.create', 'shipment', 'create', 'Create shipment'),
    ('shipment.read', 'shipment', 'read', 'Read shipment'),
    ('shipment.update', 'shipment', 'update', 'Update shipment'),
    ('shipment.assign_carrier', 'shipment', 'assign_carrier', 'Assign carrier'),

    ('rfx.create', 'rfx', 'create', 'Create RFx'),
    ('rfx.read', 'rfx', 'read', 'Read RFx'),
    ('rfx.publish', 'rfx', 'publish', 'Publish RFx'),
    ('rfx.respond', 'rfx', 'respond', 'Submit RFx response'),
    ('rfx.award', 'rfx', 'award', 'Award RFx'),

    ('document.create', 'document', 'create', 'Create document'),
    ('document.read', 'document', 'read', 'Read document'),
    ('document.sign', 'document', 'sign', 'Sign document'),

    ('billing_register.create', 'billing_register', 'create', 'Create billing register'),
    ('billing_register.read', 'billing_register', 'read', 'Read billing register'),
    ('billing_register.approve', 'billing_register', 'approve', 'Approve billing register'),
    ('billing_document.create', 'billing_document', 'create', 'Create billing document')
ON CONFLICT (code) DO NOTHING;
