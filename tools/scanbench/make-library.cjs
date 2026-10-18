// Makes a movie library of N folders, each with a copy of one tiny real video, named like Radarr does.
// node make-library.cjs <clip.mp4> <out dir> <count>
const fs = require('fs');
const path = require('path');
const [clip, out, count] = process.argv.slice(2);
const words = ['Silent', 'River', 'Night', 'Glass', 'Iron', 'Summer', 'Echo', 'Paper', 'Storm', 'Golden', 'Last', 'Hidden', 'Northern', 'Broken', 'Velvet', 'Distant'];
const nouns = ['Road', 'Garden', 'Signal', 'Harbour', 'Winter', 'Machine', 'Kingdom', 'Letter', 'Island', 'Promise', 'Shadow', 'Engine', 'Orchard', 'Tide', 'Crown', 'Voice'];
fs.mkdirSync(out, { recursive: true });
for (let i = 0; i < Number(count); i++) {
  const title = `${words[i % 16]} ${nouns[Math.floor(i / 16) % 16]} ${Math.floor(i / 256) + 1}`;
  const year = 1960 + (i % 60);
  const dir = path.join(out, `${title} (${year})`);
  fs.mkdirSync(dir, { recursive: true });
  const file = path.join(dir, `${title} (${year}).mp4`);
  if (!fs.existsSync(file)) fs.copyFileSync(clip, file);
}
console.log('made', count, 'films in', out);
