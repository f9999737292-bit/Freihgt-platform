export interface StudioNavStep {

  id: string

  label: string

  to?: string

  active?: boolean

  planned?: boolean

}



export type StudioStepId =

  | 'basics'

  | 'questionnaire'

  | 'participants'

  | 'evaluation'

  | 'communications'

  | 'validation'

  | 'publication'



export function buildStudioNavSteps(

  eventId: string,

  activeStep: StudioStepId,

  t: (key: string) => string,

): StudioNavStep[] {

  const base = `/rfx/${eventId}/studio`

  return [

    { id: 'basics', label: t('rfx.studio.steps.basics'), to: `${base}?step=basics`, active: activeStep === 'basics' },

    {

      id: 'questionnaire',

      label: t('rfx.studio.steps.questionnaire'),

      to: `${base}?step=questionnaire`,

      active: activeStep === 'questionnaire',

    },

    { id: 'participants', label: t('rfx.studio.steps.participants'), planned: true },

    { id: 'evaluation', label: t('rfx.studio.steps.evaluation'), planned: true },

    { id: 'communications', label: t('rfx.studio.steps.communications'), planned: true },

    {

      id: 'validation',

      label: t('rfx.studio.steps.validation'),

      to: `${base}?step=validation`,

      active: activeStep === 'validation',

    },

    { id: 'publication', label: t('rfx.studio.steps.publication'), planned: true },

  ]

}



export function resolveStudioStep(queryStep: unknown): StudioStepId {

  const value = String(queryStep ?? 'questionnaire')

  const allowed: StudioStepId[] = ['basics', 'questionnaire', 'validation']

  return allowed.includes(value as StudioStepId) ? (value as StudioStepId) : 'questionnaire'

}

