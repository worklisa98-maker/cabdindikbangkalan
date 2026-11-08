type Task = {
  stage: string;
  assignmentRole: string;
  schoolName: string;
  npsn: string;
  workflowStatus?: string;
};
const roles: Record<string, string> = {
  MONTHLY: "SUPERVISOR_MONTHLY",
  FIELD_STAGE_1: "FIELD_STAGE_1_ASSESSOR",
  FIELD_STAGE_2: "FIELD_STAGE_2_ASSESSOR",
  PRESENTATION: "PRESENTATION_ASSESSOR",
  NOT_FAILED_NOMINATION: "NOMINATION_ASSESSOR",
  BOTTOM5_APPEAL: "APPEAL_PANEL",
  INNOVATION: "INNOVATION_ASSESSOR",
};
export function assignedStageTabs(tasks: Task[], monthly: boolean): string[] {
  return Object.keys(roles).filter(
    (stage) =>
      (stage === "MONTHLY" && monthly) ||
      tasks.some(
        (task) => task.stage === stage && task.assignmentRole === roles[stage]
      )
  );
}
export function filterStageTasks<T extends Task>(
  tasks: T[],
  stage: string,
  search: string,
  status: string
): T[] {
  const needle = search.trim().toLocaleLowerCase("id-ID");
  return tasks.filter((task) => {
    const todo =
      !task.workflowStatus || task.workflowStatus.startsWith("RETURNED_BY_");
    return (
      task.stage === stage &&
      task.assignmentRole === roles[stage] &&
      `${task.schoolName} ${task.npsn}`
        .toLocaleLowerCase("id-ID")
        .includes(needle) &&
      (status === "ALL" ||
        (status === "TODO" && todo) ||
        (status === "SUBMITTED" && !todo) ||
        (status === "FINAL" && task.workflowStatus === "FINAL"))
    );
  });
}
