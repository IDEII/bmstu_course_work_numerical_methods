import matplotlib.pyplot as plt
import argparse
import pandas as pd
import re
from datetime import datetime

pd.set_option('display.max_rows', None)
pd.set_option('display.max_columns', None)
pd.set_option('display.width', None)
pd.set_option('display.max_colwidth', None)


def read_data_from_file(filename):
    data_dict = {}
    with open(filename, 'r') as file:
        for line in file:
            if ':' in line:
                key, values = line.split(':', 1)
                vl = values.split(',')
                vl1 = []
                for value in vl:
                    value = value.replace('\n', '').replace(' ', '')
                    try:
                        temp = int(value)
                        vl1.append(temp)
                    except ValueError:
                        pass
                data_dict[key.strip()] = vl1
    return data_dict


def read_data_from_csv(filename):
    try:
        data = pd.read_csv(filename, encoding='utf-8')
        print(f"Данные успешно загружены из {filename}")
    except FileNotFoundError:
        print(f"Файл {filename} не найден.")
        return None
    except pd.errors.EmptyDataError:
        print("Файл пуст.")
        return None
    except pd.errors.ParserError:
        print("Ошибка при разборе файла.")
        return None

    required_columns = ["Дата отчёта", "Заразилось", "Умерло", "Госпитализаций",
                        "Выздоровело", "Заразилось всего", "Умерло всего",
                        "Выздоровело всего", "Активные случаи *"]
    for column in required_columns:
        if column not in data.columns:
            print(f"Столбец '{column}' отсутствует в данных.")
            return None

    def clean_active_cases(value):
        if isinstance(value, str):
            value = value.replace('▲', ' ').replace('▼', ' ').replace(' ', '').replace('+', ' +').replace('-', ' -')
            parts = value.split(' ')
            try:
                return int(parts[0])
            except (ValueError, IndexError):
                return None
        return None
    
    def clean_dead(value):
        if isinstance(value, str):
            value = value.replace(' ', '').replace('.0', '')
            parts = value.split('.')
            try:
                return int(parts[0])
            except (ValueError, IndexError):
                return 0
        return 0

    data["Активные случаи *"] = data["Активные случаи *"].apply(clean_active_cases)
    data["Умерло всего"] = data["Умерло всего"].apply(clean_dead)
    prev_month = 12
    year = 2024
    day_idx = 0

    months = {
        'янв': 1, 'фев': 2, 'мар': 3, 'апр': 4,
        'мая': 5, 'июн': 6, 'июл': 7, 'авг': 8,
        'сен': 9, 'окт': 10, 'ноя': 11, 'дек': 12
    }
    date_objs = {}

    for i, v in data["Дата отчёта"].items():
        if isinstance(v, str):
            value = v.strip().lower()
            match = re.match(r'(\d{1,2})\s+(\w+)', value)
            if match:
                day = int(match.group(1))
                month_str = match.group(2)[:3]
                month = months.get(month_str, 1)
                if month > prev_month:
                    year -= 1
                prev_month = month
                try:
                    date_obj = datetime(year, month, day)
                    date_objs[i] = date_obj
                except ValueError:
                    date_objs[i] = pd.NaT
            else:
                date_objs[i] = pd.NaT
        else:
            date_objs[i] = pd.NaT

    valid_dates = [d for d in date_objs.values() if isinstance(d, datetime)]
    if valid_dates:
        latest_date = min(valid_dates)
        for i, date_obj in date_objs.items():
            if isinstance(date_obj, datetime):
                diff = (latest_date - date_obj).days
                data.at[i, "Дата отчёта"] = -diff
            else:
                data.at[i, "Дата отчёта"] = pd.NaT
    else:
        data["Дата отчёта"] = pd.NaT


    data = data.sort_values(by='Дата отчёта')

    numeric_columns = ["Заразилось", "Умерло", "Госпитализаций",
                       "Выздоровело", "Заразилось всего", "Умерло всего",
                       "Выздоровело всего"]

    for column in numeric_columns:
        data[column] = data[column].astype(str).str.replace(r'\s', '', regex=True).replace(' ', '').replace('—', '0')
        try:
            data[column] = data[column].astype(int)
        except ValueError:
            print(f"Не удалось преобразовать значения в столбце '{column}' в целые числа.")

    return data


def plot_data_csv(ax, data):
    ax.scatter(data['Дата отчёта'], data['Активные случаи *'], label='Активные случаи', color='blue', s=10)
    
    ax.set_xlabel('Дата отчёта')
    ax.set_ylabel('Количество активных случаев')
    ax.set_title('Активные случаи по датам')
    ax.legend()
    ax.grid(True)


def plot_data_txt(ax, data):
    t = min(len(v) for v in data.values() if isinstance(v, list) and v)
    if t == 0:
        print("Нет данных для построения графика из TXT файла.")
        return

    labels = ['S', 'E', 'SC', 'HI', 'IH', 'Imm', 'D']
    for label in labels:
        if label in data:
            if label == 'IH':
                temp = [0] * len(data['IH'][:t])
                for i in range(len(data['IH'][:t])):
                    temp[i] = data['IH'][i] + data['IHosp'][i]  
                ax.plot(range(t), temp, label=label, linestyle='solid')
            else:
                ax.plot(range(t), data[label][:t], label=label, linestyle='solid')

    ax.set_xlabel('Дни')
    ax.set_ylabel('Количество человек')
    ax.set_title('Состояние людей')
    ax.legend()
    ax.grid(True)


def plots(csv, txt):
    import matplotlib.pyplot as plt

    if csv is None or txt is None:
        print("Недостаточно данных для построения комбинированного графика.")
        return

    active_cases = csv['Активные случаи *']

    report_days = csv['Дата отчёта']
    dead_by_days = csv['Умерло всего']
    txt_days = max(len(v) for v in txt.values() if isinstance(v, list) and v)
    csv_days = report_days.max() if not report_days.isnull().all() else 0

    max_day = max(txt_days, csv_days)
    x_range = range(max_day + 1-500)

    plt.figure(figsize=(14, 8))

    # labels = ['S', 'E', 'SC', 'HI', 'IH', 'R', 'Imm', 'D']
    labels = ['Imm']

    for label in labels:
        if label in txt:
            data_length = len(txt[label])

            if label == 'S':
                Sdata = [0] * len(txt['S'])
                for i in range(len(txt['S'])):
                    Sdata[i] = txt['S'][i]
                plt.plot(range, Sdata, label='Восприимчивые', linestyle='solid')
            elif label == 'IH':
                temp = [0] * len(txt['IH'])
                for i in range(len(txt['IH'])):
                    temp[i] = txt['IH'][i] + txt['IHosp'][i]
                plt.plot(range(data_length), temp, label='Выявленные инфицированные', linestyle='solid')
            elif label == 'HI':
                temp = [0] * len(txt['HI'])
                for i in range(len(txt['HI'])):
                    temp[i] = txt['HI'][i]
                plt.plot(range(data_length), temp, label='Скрытозаражен', linestyle='solid')
            elif label == 'Imm':
                temp = [0] * len(txt['Imm'])
                for i in range(len(txt['Imm'])):
                    temp[i] = txt['Imm'][i]
                plt.plot(range(data_length), temp, label='Вакцинрованные', linestyle='solid')
            elif label == 'D':
                plt.plot(range(data_length), txt[label], label='Умершие (Модель)', linestyle='solid')
            elif label == 'E':
                temp = [0] * len(txt['E'])
                for i in range(len(txt['E'])):
                    temp[i] = txt['E'][i+5]
                plt.plot(range(data_length-5), temp, label='Латентный', linestyle='solid')
            elif label == 'SC':
                temp = [0] * len(txt['SC'])
                for i in range(len(txt['SC'])):
                    temp[i] = txt['SC'][i+5]
                print(temp[:10])
                plt.plot(range(data_length), temp, label='Субклинический', linestyle='solid')
            else:
                plt.plot(range(data_length), temp, label=label, linestyle='solid')
    
    # plt.scatter(
    #     report_days,
    #     active_cases,
    #     label='Активные случаи',
    #     color='blue',
    #     s=20,
    #     edgecolors='w',
    #     alpha=0.7
    # )
    # plt.scatter(
    #     report_days,
    #     dead_by_days,
    #     label='Умершие (Статистика)',
    #     color='black',
    #     s=10,
    #     edgecolors='w',
    #     alpha=0.7
    # )


    plt.xlabel('Дни', fontsize=12)
    plt.ylabel('Количество людей', fontsize=12)
    plt.title('Восприимчевые агенты', fontsize=16)
    plt.legend(fontsize=10)
    plt.grid(False)
    plt.xticks(x_range)


def main():
    parser = argparse.ArgumentParser(description='Чтение данных из файлов для построения графиков.')
    parser.add_argument('filenames', type=str, nargs='+', help='Имена файлов для чтения данных (csv и/или txt)')

    args = parser.parse_args()

    csv_data = None
    txt_data = None

    for filename in args.filenames:
        if filename.lower().endswith('.csv'):
            csv_data = read_data_from_csv(filename)
        elif filename.lower().endswith('.txt'):
            txt_data = read_data_from_file(filename)
        else:
            print(f"Неподдерживаемый формат файла: {filename}")

    if csv_data is None and txt_data is None:
        print("Нет данных для построения графиков.")
        return

    if csv_data.bool and txt_data:
        plots(csv_data, txt_data)
    elif txt_data:
        fig, ax = plt.subplots(figsize=(12, 8))
        plot_data_txt(ax, txt_data)
    elif csv_data.bool:
        fig, ax = plt.subplots(figsize=(12, 8))
        plot_data_csv(ax, csv_data)
    
    plt.tight_layout()
    plt.show()


if __name__ == "__main__":
    main()