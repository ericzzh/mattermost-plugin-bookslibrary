export function Template(temp: string, ...values: any[]) {
    const strings = temp.split("%v");
    const result:any[] = [];
    strings.forEach((string, i) => {
        result.push(string, values[i]);
    });
    return result.join("");
}
