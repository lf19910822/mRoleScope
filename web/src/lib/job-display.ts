import type { EducationRequirement, ExperienceRequirement } from '@/api/jd-api'

const educationLabels = {
  associate_or_lower: '专科',
  bachelor: '本科',
  master: '硕士',
  doctor: '博士',
} as const

function educationLevelLabel(level: keyof typeof educationLabels): string {
  return educationLabels[level]
}

export function formatEducation(requirement: EducationRequirement | undefined): string {
  if (requirement === undefined || !requirement.explicit_requirement) {
    return '未明确学历要求'
  }
  if (requirement.lower_bound !== undefined && requirement.upper_bound !== undefined) {
    if (requirement.lower_bound === requirement.upper_bound) {
      return educationLevelLabel(requirement.lower_bound)
    }
    return `${educationLevelLabel(requirement.lower_bound)}至${educationLevelLabel(requirement.upper_bound)}`
  }
  if (requirement.upper_bound !== undefined) {
    return requirement.upper_bound === 'associate_or_lower'
      ? '专科及以下'
      : `${educationLevelLabel(requirement.upper_bound)}及以下`
  }
  if (requirement.lower_bound !== undefined) {
    return requirement.lower_bound === 'associate_or_lower'
      ? '专科及以上'
      : `${educationLevelLabel(requirement.lower_bound)}及以上`
  }
  return '未明确学历要求'
}

export function formatExperience(requirement: ExperienceRequirement | undefined): string {
  if (
    requirement === undefined ||
    !requirement.explicit_requirement ||
    requirement.minimum_years === undefined
  ) {
    return '未明确工作年限'
  }
  return `最低 ${requirement.minimum_years} 年`
}

export function formatActivityState(state: 'effective' | 'confirmed_closed'): string {
  return state === 'effective' ? '仍有效' : '已确认下线'
}

export function formatFirstDiscoveredDate(value: string): string {
  const [year, month, day] = value.split('-')
  return year !== undefined && month !== undefined && day !== undefined
    ? `${year}.${month}.${day}`
    : value
}
