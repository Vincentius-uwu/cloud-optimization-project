#!/bin/bash

# Config des dossiers et fichiers
BIN_DIR="bin"
DATA_DIR="data"
OUTPUT_CSV="${DATA_DIR}/benchmark_local.csv"

# Création du dossier data s'il n'existe pas
mkdir -p "$DATA_DIR"

# Initialisation du fichier CSV avec l'en-tête s'il est vide ou inexistant
if [ ! -f "$OUTPUT_CSV" ]; then
    echo "workload,size,run,runtime_s,max_rss_kb" > "$OUTPUT_CSV"
fi

# Fonction principale de benchmarking
run_benchmark() {
    local workload=$1
    local size=$2
    local runs=$3
    local executable="${BIN_DIR}/${workload}"

    # Vérification de l'existence de l'exécutable
    if [ ! -x "$executable" ]; then
        echo "[Erreur] L'exécutable '$executable' n'existe pas ou n'est pas exécutable." >&2
        return 1
    fi

    echo "Lancement: $workload | Taille: $size | Répétitions: $runs"

    for (( i=1; i<=runs; i++ )); do
        # Fichier temporaire pour stocker la sortie de /usr/bin/time
        tmp_time=$(mktemp)

        # Exécution avec GNU time
        # %e = temps écoulé en secondes (wall-clock)
        # %M = Maximum Resident Set Size en Ko
        /usr/bin/time -f "%e,%M" -o "$tmp_time" "$executable" --size "$size" > /dev/null 2>&1
        
        local exit_code=$?

        if [ $exit_code -ne 0 ]; then
            echo "  [Erreur] L'exécution a échoué pour $workload (Run $i)" >&2
            rm -f "$tmp_time"
            continue
        fi

        # Lecture et extraction des résultats
        local result=$(cat "$tmp_time")
        local runtime_s=$(echo "$result" | cut -d',' -f1)
        local max_rss_kb=$(echo "$result" | cut -d',' -f2)

        # Ajout de la ligne dans le CSV
        echo "${workload},${size},${i},${runtime_s},${max_rss_kb}" >> "$OUTPUT_CSV"
        
        echo "  -> Run $i OK : ${runtime_s}s, ${max_rss_kb} Ko"
        
        # Nettoyage
        rm -f "$tmp_time"
    done
}

# === Campagne de tests (Partie 6 du TD) ===
echo "--- Début de la campagne de benchmark ---"

# CPU Task
run_benchmark "cpu_task" 200000 3
run_benchmark "cpu_task" 500000 3
run_benchmark "cpu_task" 1000000 3

# Memory Task
run_benchmark "memory_task" 128 3
run_benchmark "memory_task" 256 3
run_benchmark "memory_task" 512 3

# Matrix Task
run_benchmark "matrix_task" 200 3
run_benchmark "matrix_task" 400 3
run_benchmark "matrix_task" 600 3

echo "--- Fin. Les résultats sont enregistrés dans $OUTPUT_CSV ---"
