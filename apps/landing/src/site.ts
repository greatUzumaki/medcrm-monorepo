// Company data in one place. Placeholder values — replace before launch.
export const site = {
  brand: 'OpenMedAI',
  phone: '+7 960 288-80-08',
  phoneHref: 'tel:+79602888008',
  email: 'hello@openmedai.ru',
  telegram: 'https://t.me/openmedai',
  telegramLabel: '@openmedai',
  // Lead endpoint of apps/api (POST /public/leads). Unset → form reports it is not connected.
  leadsEndpoint: import.meta.env.PUBLIC_LEADS_ENDPOINT as string | undefined,
};
