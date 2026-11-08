import test from "node:test";
import assert from "node:assert/strict";
import { assignedStageTabs, filterStageTasks } from "../lib/gaspol-stage-navigation.ts";

const tasks = [
 { assignmentId: 1, stage: "FIELD_STAGE_1", assignmentRole: "FIELD_STAGE_1_ASSESSOR", schoolName: "SMA Bangkalan", npsn: "20500001", workflowStatus: "" },
 { assignmentId: 2, stage: "FIELD_STAGE_1", assignmentRole: "FIELD_STAGE_1_ASSESSOR", schoolName: "SMK Kamal", npsn: "20500002", workflowStatus: "FINAL" },
 { assignmentId: 3, stage: "PRESENTATION", assignmentRole: "PRESENTATION_ASSESSOR", schoolName: "SMA Bangkalan", npsn: "20500001", workflowStatus: "RETURNED_BY_MASTER" },
 { assignmentId: 4, stage: "NOT_FAILED_NOMINATION", assignmentRole: "NOMINATION_ASSESSOR", schoolName: "SMA Arosbaya", npsn: "20500003", workflowStatus: "" },
];
test("tabs follow stage AND assignment role, with monthly only for monthly assignees", () => {
 assert.deepEqual(assignedStageTabs(tasks, false), ["FIELD_STAGE_1", "PRESENTATION", "NOT_FAILED_NOMINATION"]);
 assert.equal(assignedStageTabs(tasks, true)[0], "MONTHLY");
 assert.deepEqual(assignedStageTabs([], false), []);
 assert.deepEqual(assignedStageTabs([{ ...tasks[0], assignmentRole: "KOOR_GASPOL" }], false), []);
});
test("multi-school list can search names or NPSN without crossing stages", () => {
 assert.equal(filterStageTasks(tasks, "FIELD_STAGE_1", "", "ALL").length, 2);
 assert.deepEqual(filterStageTasks(tasks, "FIELD_STAGE_1", " kamAL ", "ALL").map(t => t.assignmentId), [2]);
 assert.deepEqual(filterStageTasks(tasks, "FIELD_STAGE_1", "20500001", "TODO").map(t => t.assignmentId), [1]);
 assert.equal(filterStageTasks(tasks, "INNOVATION", "", "ALL").length, 0);
});
test("returned work remains actionable, submitted and final filters are separate", () => {
 assert.equal(filterStageTasks(tasks, "PRESENTATION", "", "TODO").length, 1);
 assert.deepEqual(filterStageTasks(tasks, "FIELD_STAGE_1", "", "FINAL").map(t => t.assignmentId), [2]);
 assert.equal(filterStageTasks(tasks, "PRESENTATION", "", "SUBMITTED").length, 0);
});
