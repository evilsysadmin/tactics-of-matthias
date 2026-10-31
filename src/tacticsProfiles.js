// Tactics owns these combat classes and talent overrides; MM3 uses separate rules.
// Pure rules source; migrated from Chronicles at git blob fa0a797c0b4f13b0b44d9c183897e5a5e43f63cb.
const CLASS_PROFILES = Object.freeze({
  matthias: Object.freeze({
    className: 'Espadachín',
    weaponName: 'Espada corta',
    attackName: 'Estocada teutona',
    attackKind: 'melee',
    kindLabel: 'cuerpo a cuerpo',
    attackPattern: 'adjacent',
    reach: 1,
    damage: 2,
    abilityName: 'Ruptura teutona',
    abilityKind: 'burst',
    abilityLabel: 'golpe de ejecución',
    abilityDamage: 4,
  }),
  rook: Object.freeze({
    className: 'Guardiana',
    weaponName: 'Maza de torre',
    attackName: 'Embestida de torre',
    attackKind: 'heavy',
    kindLabel: 'línea pesada',
    attackPattern: 'orthogonal',
    reach: 2,
    damage: 2,
    abilityName: 'Martillo de asedio',
    abilityKind: 'burst',
    abilityLabel: 'impacto pesado',
    abilityDamage: 4,
  }),
  bishop: Object.freeze({
    className: 'Taumaturgo',
    weaponName: 'Farol rúnico',
    attackName: 'Rayo diagonal',
    attackKind: 'spell',
    kindLabel: 'conjuro diagonal',
    attackPattern: 'diagonal',
    reach: 4,
    damage: 2,
    abilityName: 'Luz del farol',
    abilityKind: 'heal',
    abilityLabel: 'conjuro de apoyo',
    abilityHeal: 2,
  }),
  knight: Object.freeze({
    className: 'Hostigador',
    weaponName: 'Ballesta de estribo',
    attackName: 'Virote largo',
    attackKind: 'ranged',
    kindLabel: 'arma a distancia',
    attackPattern: 'line',
    reach: 3,
    damage: 1,
    abilityName: 'Salva de virotes',
    abilityKind: 'volley',
    abilityLabel: 'salva a distancia',
    abilityDamage: 2,
    abilityMaxTargets: 2,
  }),
});

const FALLBACK_PROFILE = Object.freeze({
  className: 'Aventurero',
  weaponName: 'Arma improvisada',
  attackName: 'Golpe',
  attackKind: 'melee',
  kindLabel: 'cuerpo a cuerpo',
  attackPattern: 'adjacent',
  reach: 1,
  damage: 1,
  abilityName: 'Recurso desesperado',
  abilityKind: 'burst',
  abilityLabel: 'maniobra',
  abilityDamage: 2,
});

export function chroniclesTacticsProfile(memberId) {
  return CLASS_PROFILES[memberId] || FALLBACK_PROFILE;
}

export function chroniclesTacticsEffectiveProfile(state, memberId) {
  const base = chroniclesTacticsProfile(memberId);
  const overrides = state?.rpgModifiers?.[memberId]?.profileOverrides;
  if (!overrides || typeof overrides !== 'object') return base;
  return { ...base, ...overrides };
}
