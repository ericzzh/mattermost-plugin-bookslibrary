export function Template(temp: string, ...values: any[]) {
    const strings = temp.split("%v");
    const result: any[] = [];
    strings.forEach((string, i) => {
        result.push(string, values[i]);
    });
    return result.join("");
}

export function ExistedInArray<T>(keys: Array<T>, values: Array<T>) {
    for (const key of keys) {
        if (values.findIndex((value) => key === value) !== -1) {
            return true;
        }
    }

    return false
}
