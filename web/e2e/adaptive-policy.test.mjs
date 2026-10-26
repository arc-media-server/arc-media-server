import test from 'node:test';
import assert from 'node:assert/strict';
import { AdaptiveQuality } from '../src/lib/adaptive-quality.ts';

const rates = [20000,12000,8000,4000,3000,2000,1500,720];
const observation = (now, patch = {}) => ({ now, active: true, bufferSeconds: 0,
  waiting: true, remainingSeconds: 300, playbackRate: 1, currentLimit: 0,
  originalKbps: 8000, ceilingKbps: 0, ...patch });

test('startup grace, sustained pressure and downshift cooldown avoid a restart per stall', () => {
  const p = new AdaptiveQuality(rates); p.reset(0);
  assert.equal(p.decide(observation(11000)), null);
  assert.equal(p.decide(observation(12000)), null);
  assert.equal(p.decide(observation(18000)), 4000);
  assert.equal(p.decide(observation(19000, { currentLimit: 4000 })), null);
  assert.equal(p.decide(observation(25000, { currentLimit: 4000 })), null);
  assert.equal(p.decide(observation(38000, { currentLimit: 4000 })), 3000);
});

test('measured delivery includes wait time and picks a conservative viable floor', () => {
  const p = new AdaptiveQuality(rates);p.reset(0);
  p.observe(12000,2000000,16000,2); // 1 Mbps delivery, 8 Mbps payload.
  assert.equal(p.decide(observation(12000,{waiting:false})),null);
  assert.equal(p.decide(observation(18000,{waiting:false})),720);
  assert.equal(p.deliveryKbps,1000);
  p.observe(19000,NaN,1,1);p.observe(19000,100,1,1);
  assert.equal(p.deliveryKbps,1000);
});

test('upgrades require fresh repeated evidence, a deep buffer and 45s stability', () => {
  const p=new AdaptiveQuality(rates);p.reset(0);let target=null;
  for(let now=11000;now<=57000;now+=1000){
    p.observe(now,1000000,200,8);
    target=p.decide(observation(now,{currentLimit:720,bufferSeconds:30,waiting:false}));
    if(now<57000)assert.equal(target,null);
  }
  assert.equal(target,1500);
  p.observe(60000,1000000,200,8);
  assert.equal(p.decide(observation(60000,{currentLimit:1500,bufferSeconds:30,waiting:false})),null);
  assert.equal(p.decide(observation(90000,{currentLimit:1500,bufferSeconds:30,waiting:false})),null); // stale samples
});

test('pause, seek, hidden/receiver states, near-end and buffer gaps reset pressure', () => {
  const p=new AdaptiveQuality(rates);p.reset(0);
  p.decide(observation(12000));
  assert.equal(p.decide(observation(18000,{active:false})),null);
  assert.equal(p.decide(observation(29000)),null);
  assert.equal(p.decide(observation(30000)),null);
  assert.equal(p.decide(observation(36000,{remainingSeconds:5})),null);
  assert.equal(p.decide(observation(50000,{bufferSeconds:NaN})),null);
});

test('original upgrades stay under the independent server ceiling and playback-rate headroom', () => {
  const p=new AdaptiveQuality(rates);p.reset(0);let target=null;
  for(let now=11000;now<=57000;now+=1000){p.observe(now,1000000,200,8);target=p.decide(observation(now,{currentLimit:1500,ceilingKbps:2000,bufferSeconds:30,waiting:false,playbackRate:2}));}
  assert.equal(target,0); // Original means the server's 2 Mbps ceiling, not uncapped 8 Mbps.
  const low=new AdaptiveQuality(rates);low.reset(0);
  low.observe(12000,2000000,16000,2);
  low.decide(observation(12000,{ceilingKbps:500}));
  assert.equal(low.decide(observation(18000,{ceilingKbps:500})),null); // no request above an already tighter floor
});

test('at most four automatic changes in two minutes', () => {
  const p=new AdaptiveQuality(rates);p.reset(0);let current=0;
  for(const [start,end,want] of [[12000,18000,4000],[32000,38000,3000],[52000,58000,2000],[72000,78000,1500]]){
    p.decide(observation(start,{currentLimit:current}));
    assert.equal(p.decide(observation(end,{currentLimit:current})),want);current=want;
  }
  p.decide(observation(92000,{currentLimit:current}));
  assert.equal(p.decide(observation(98000,{currentLimit:current})),null);
  assert.equal(p.decide(observation(139000,{currentLimit:current})),720);
});
