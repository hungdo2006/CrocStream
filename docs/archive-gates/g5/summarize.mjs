// Independently verify saved numeric evidence, never read capture images.
import {readFileSync} from 'node:fs'
const file=process.argv[2]
const bytes=readFileSync(file)
const records=bytes.toString(bytes[0]===0xff&&bytes[1]===0xfe?'utf16le':'utf8').replace(/^\uFEFF/,'').trim().split(/\r?\n/).map(JSON.parse)
const start=records.find(r=>r.phase==='stress-start'),r=records.find(r=>r.phase==='stress-result'),end=records.at(-1)
if(!r||!start||end.status!=='G5_LOCAL_UI_PASS'||r.seconds<start.duration||r.stopMs>3000||end.pageErrors!==0)throw Error('Incomplete evidence')
const q=(values,p)=>{const sorted=values.filter(Number.isFinite).sort((a,b)=>a-b);return sorted[Math.floor((sorted.length-1)*p)]}
for(const [key,stats] of Object.entries(r.summary)){
  if(q(r.samples.map(s=>s[key]),.5)!==stats.p50||q(r.samples.map(s=>s[key]),.95)!==stats.p95)throw Error('Quantile mismatch')
}
for(const s of r.samples)if(s.savings!==null&&Math.abs(s.savings-100*(1-s.sentKiB/s.baselineKiB))>1e-8)throw Error('Baseline mismatch')
const resources={}
for(const [role,initial,after,frames] of [
  ['host',start.initialHost,r.afterHost,r.hostStats.counters.emitted],
  ['viewer',start.initialViewer,r.afterViewer,r.viewerStats.counters.decoded],
]){
  const first=r.resources[0][role],last=r.resources.at(-1)[role],ms=r.resources.at(-1).ms-r.resources[0].ms
  if(after.gdiObjects!==initial.gdiObjects||after.goroutines!==initial.goroutines)throw Error('Cleanup baseline mismatch')
  const series=r.resources.map(s=>s[role])
  resources[role]={
    samples:series.length,firstWorkingSetMiB:first.workingSetBytes/2**20,lastWorkingSetMiB:last.workingSetBytes/2**20,
    maxSampledWorkingSetMiB:Math.max(...series.map(s=>s.workingSetBytes))/2**20,
    maxSampledHeapMiB:Math.max(...series.map(s=>s.heapBytes))/2**20,
    minHandles:Math.min(...series.map(s=>s.handles)),maxHandles:Math.max(...series.map(s=>s.handles)),
    handlesBefore:initial.handles,handlesAfter:after.handles,
    goroutinesBefore:initial.goroutines,goroutinesAfter:after.goroutines,
    cpuOneCorePercent:100*(last.cpuMs-first.cpuMs)/ms,
    cpuMachineCapacityPercent:100*(last.cpuMs-first.cpuMs)/ms/first.logicalCPUs,
    // Includes preparation, all process work and final snapshot skew; explicitly approximate.
    approximateProcessAllocsPerVideo:(after.allocations-initial.allocations)/frames,
    approximateProcessAllocatedMiBPerVideo:(after.allocatedBytes-initial.allocatedBytes)/frames/2**20,
  }
}
const firstTwoMinutes=r.samples.filter(s=>s.ms<=120000)
const twoMinute={}
for(const key of ['capture','emitted','decoded','rendered']){
  const values=firstTwoMinutes.map(s=>s[key])
  twoMinute[key]={samples:values.length,p50:q(values,.5),p95:q(values,.95),mean:values.reduce((a,b)=>a+b,0)/values.length}
}
console.log(JSON.stringify({status:'G5_EVIDENCE_VERIFIED',seconds:r.seconds,stopMs:r.stopMs,checks:end.checks.length,
  closedListeners:end.closedListeners,summary:r.summary,firstTwoMinuteWindow:twoMinute,resources},null,2))
