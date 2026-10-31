import assert from 'node:assert/strict';
import test from 'node:test';
import {
  chroniclesTacticsProfile,
  chroniclesTacticsEffectiveProfile,
} from '../src/tacticsProfiles.js';

test('four tactical class profiles retain their attacks and abilities', () => {
  assert.equal(chroniclesTacticsProfile('matthias').attackPattern, 'adjacent');
  assert.equal(chroniclesTacticsProfile('rook').attackPattern, 'orthogonal');
  assert.equal(chroniclesTacticsProfile('rook').reach, 2);
  assert.equal(chroniclesTacticsProfile('bishop').attackPattern, 'diagonal');
  assert.equal(chroniclesTacticsProfile('bishop').abilityKind, 'heal');
  assert.equal(chroniclesTacticsProfile('knight').attackPattern, 'line');
  assert.equal(chroniclesTacticsProfile('knight').abilityMaxTargets, 2);
});

test('unrecognized member receives the previous safe fallback profile', () => {
  assert.deepEqual(
    [chroniclesTacticsProfile('unknown').className, chroniclesTacticsProfile('unknown').damage],
    ['Aventurero', 1],
  );
});

test('talent overrides are isolated per member and do not mutate base data', () => {
  const original = chroniclesTacticsProfile('matthias');
  const state = {
    rpgModifiers: {
      matthias: { profileOverrides: { attackName: 'Estocada veterana', damage: 7 } },
    },
  };
  const override = chroniclesTacticsEffectiveProfile(state, 'matthias');
  assert.equal(override.attackName, 'Estocada veterana');
  assert.equal(override.damage, 7);
  assert.equal(override.className, 'Espadachín');
  assert.equal(chroniclesTacticsEffectiveProfile({}, 'matthias'), original);
  assert.equal(original.attackName, 'Estocada teutona');
  assert.equal(original.damage, 2);
  assert.ok(Object.isFrozen(original));
});
