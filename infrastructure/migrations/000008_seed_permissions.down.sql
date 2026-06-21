DELETE FROM core.permissions
WHERE code IN (
    'company.create','company.read','company.update',
    'user.invite','user.read','user.update',
    'transport_order.create','transport_order.read','transport_order.update','transport_order.submit',
    'shipment.create','shipment.read','shipment.update','shipment.assign_carrier',
    'rfx.create','rfx.read','rfx.publish','rfx.respond','rfx.award',
    'document.create','document.read','document.sign',
    'billing_register.create','billing_register.read','billing_register.approve','billing_document.create'
);
